package deployer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/versaDeploy/internal/artifact"
	"github.com/user/versaDeploy/internal/builder"
	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/fsutil"
	"github.com/user/versaDeploy/internal/git"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/state"
	"golang.org/x/sync/errgroup"
)

// Deployer orchestrates the entire deployment process
type Deployer struct {
	cfg            *config.Config
	env            *config.Environment
	envName        string
	repoPath       string
	dryRun         bool
	initialDeploy  bool
	force          bool
	skipDirtyCheck bool
	log            *logger.Logger

	// PostDeployConfirm is called before post_deploy hooks on an initial deploy.
	// Return true to run hooks, false to skip them. If nil, hooks always run.
	PostDeployConfirm func() bool
}

// NewDeployer creates a new deployer
func NewDeployer(cfg *config.Config, envName, repoPath string, dryRun, initialDeploy, force, skipDirtyCheck bool, log *logger.Logger) (*Deployer, error) {
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	return &Deployer{
		cfg:            cfg,
		env:            env,
		envName:        envName,
		repoPath:       repoPath,
		dryRun:         dryRun,
		initialDeploy:  initialDeploy,
		force:          force,
		skipDirtyCheck: skipDirtyCheck,
		log:            log,
	}, nil
}

// remote joins path elements under the environment's remote_path.
func (d *Deployer) remote(parts ...string) string {
	return filepath.ToSlash(filepath.Join(append([]string{d.env.RemotePath}, parts...)...))
}

// chunkSize returns the archive chunk size in bytes.
func (d *Deployer) chunkSize() int64 {
	if d.env.ChunkSizeMB <= 0 {
		return 10 << 20
	}
	return int64(d.env.ChunkSizeMB) << 20
}

// deadline returns a func that errors once deploy_timeout has elapsed.
func (d *Deployer) deadline() func() error {
	secs := d.env.DeployTimeout
	if secs <= 0 {
		secs = 600 // default 10 minutes
	}
	end := time.Now().Add(time.Duration(secs) * time.Second)
	return func() error {
		if time.Now().After(end) {
			return fmt.Errorf("deployment aborted: timeout of %ds exceeded", secs)
		}
		return nil
	}
}

// Deploy executes the full deployment workflow
func (d *Deployer) Deploy() (returnErr error) {
	if d.env.Local {
		return d.deployLocal()
	}

	startTime := time.Now()
	d.log.Info("Starting deployment to %s", d.envName)
	checkTimeout := d.deadline()

	// Notification defer: send webhook on success or failure
	var releaseVer, commitRef string
	defer func() {
		if !d.dryRun {
			d.sendNotification(releaseVer, commitRef, returnErr, time.Since(startTime))
		}
	}()

	tmpRepo, commitHash, err := d.prepareRepo()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRepo)
	commitRef = commitHash

	sshClient, unlock, err := d.connectAndLock()
	if err != nil {
		return err
	}
	defer unlock()

	previousLock, err := d.fetchPreviousLock(sshClient)
	if err != nil {
		return err
	}

	d.log.Info("Calculating changes...")
	cs, err := d.detectChanges(tmpRepo, previousLock)
	if err != nil {
		return err
	}
	cs.Force = d.force

	if !cs.HasChanges() && !d.force {
		d.log.Info("No changes detected - skipping deployment")
		return nil
	}
	if d.force {
		d.log.Info("Force redeploy requested - bypassing change detection")
	}

	d.log.Info("Changes detected: %d PHP, %d Twig, %d Go, %d Frontend, %d Python, %d other files",
		len(cs.PHPFiles), len(cs.TwigFiles), len(cs.GoFiles), len(cs.FrontendFiles), len(cs.PythonFiles), len(cs.OtherFiles))

	if d.dryRun {
		d.printChanges(cs)
		return nil
	}

	if err := d.checkGoTarget(sshClient); err != nil {
		return err
	}
	if err := checkTimeout(); err != nil {
		return err
	}
	artifactDir, releaseVersion, err := d.buildFrom(tmpRepo, commitHash, cs)
	if err != nil {
		return err
	}
	releaseVer = releaseVersion

	a := &PrebuiltArtifact{ReleaseVersion: releaseVersion, CommitHash: commitHash, ChangeSet: cs, artifactDir: artifactDir}
	defer a.Cleanup()
	return d.ship(sshClient, a, previousLock, checkTimeout)
}

// printChanges lists the files a deploy would ship (dry run / `versa diff`).
func (d *Deployer) printChanges(cs *changeset.ChangeSet) {
	d.log.Info("DRY RUN - these changes would be deployed:")
	groups := []struct {
		name  string
		files []string
	}{
		{"PHP", cs.PHPFiles}, {"Twig", cs.TwigFiles}, {"Go", cs.GoFiles},
		{"Frontend", cs.FrontendFiles}, {"Python", cs.PythonFiles}, {"Other", cs.OtherFiles},
	}
	for _, g := range groups {
		if len(g.files) == 0 {
			continue
		}
		d.log.Info("%s (%d):", g.name, len(g.files))
		for _, f := range g.files {
			d.log.Info("  %s", f)
		}
	}
	deps := []struct {
		name    string
		changed bool
	}{
		{"composer", cs.ComposerChanged}, {"package.json", cs.PackageChanged},
		{"go.mod", cs.GoModChanged}, {"python requirements", cs.RequirementsChanged}, {"routes", cs.RoutesChanged},
	}
	for _, dep := range deps {
		if dep.changed {
			d.log.Info("Dependencies changed: %s (will be rebuilt)", dep.name)
		}
	}
}

// connectAndLock opens the SSH connection and takes the remote deploy lock.
// The returned func releases the lock and closes the connection.
func (d *Deployer) connectAndLock() (*ssh.Client, func(), error) {
	d.log.Info("Connecting to %s@%s...", d.env.SSH.User, d.env.SSH.Host)
	c, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return nil, nil, verserrors.Wrap(err)
	}

	lockDir := d.remote(".versa.lock")
	d.log.Debug("Acquiring deployment lock...")
	if err := c.AcquireLock(lockDir); err != nil {
		c.Close()
		return nil, nil, err
	}
	return c, func() {
		d.log.Debug("Releasing deployment lock...")
		if err := c.ReleaseLock(lockDir); err != nil {
			d.log.Warn("Failed to release deployment lock: %v", err)
		}
		c.Close()
	}, nil
}

// fetchPreviousLock reads deploy.lock from the server; nil on an initial deploy.
func (d *Deployer) fetchPreviousLock(c *ssh.Client) (*state.DeployLock, error) {
	lockPath := d.remote("deploy.lock")
	exists, err := c.FileExists(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check deploy.lock: %w", err)
	}
	if !exists {
		if !d.initialDeploy {
			return nil, verserrors.Wrap(fmt.Errorf("deploy.lock not found on remote server"))
		}
		d.log.Info("First deployment detected (--initial-deploy)")
		return nil, nil
	}

	d.log.Debug("Fetching deploy.lock from remote...")
	data, err := c.ReadRemoteBytes(lockPath, 256<<20)
	if err != nil {
		return nil, err
	}
	lock, err := state.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deploy.lock: %w", err)
	}
	return lock, nil
}

// ship uploads a built release, activates it and records it in deploy.lock.
// Shared by Deploy and DeployWithArtifact.
func (d *Deployer) ship(c *ssh.Client, a *PrebuiltArtifact, previousLock *state.DeployLock, checkTimeout func() error) error {
	if err := checkTimeout(); err != nil {
		return err
	}
	d.log.Info("Uploading artifact to remote server...")
	releasesDir := d.remote("releases")
	finalDir := d.remote("releases", a.ReleaseVersion)
	if err := c.MkdirAll(releasesDir); err != nil {
		return err
	}
	if err := d.uploadRelease(c, a, previousLock, finalDir); err != nil {
		return err
	}

	// Shared paths, reused dependencies and preserved paths
	if err := d.handleSharedPaths(c, finalDir); err != nil {
		return err
	}
	if previousLock != nil {
		if err := d.reuseDependencies(c, previousLock.LastDeploy.ReleaseDir, finalDir, a.ChangeSet); err != nil {
			return err
		}
		if err := d.handlePreservedPaths(c, previousLock.LastDeploy.ReleaseDir, finalDir); err != nil {
			return err
		}
	}

	// Validate runtime artifacts before activating symlink
	if err := d.validateRuntimeArtifacts(c, finalDir, a.ChangeSet); err != nil {
		return err
	}

	// pre_deploy_server hooks (non-fatal, before symlink switch)
	if err := checkTimeout(); err != nil {
		return err
	}
	d.executePreDeployServer(c, finalDir)

	// Atomic symlink switch (absolute target is more robust)
	if err := checkTimeout(); err != nil {
		return err
	}
	d.log.Info("Activating release...")
	currentSymlink := d.remote("current")
	d.log.Info("  Linking: %s -> %s", currentSymlink, finalDir)
	if err := c.CreateSymlink(finalDir, currentSymlink); err != nil {
		return err
	}

	// Reload services (PHP-FPM, Apache/Nginx, etc.) to clear caches
	d.executeServicesReload(c)

	// post_deploy hooks (after symlink switch)
	skipPostDeploy := false
	if d.initialDeploy && len(d.env.PostDeploy) > 0 && d.PostDeployConfirm != nil {
		if !d.PostDeployConfirm() {
			d.log.Info("Post-deploy hooks skipped by user (initial deploy)")
			skipPostDeploy = true
		}
	}
	if !skipPostDeploy {
		if err := d.executePostDeployHooks(c, finalDir, previousLock); err != nil {
			return err
		}
	}

	if err := d.performHealthCheck(previousLock, c, finalDir); err != nil {
		return err
	}

	d.log.Info("Updating deploy.lock...")
	cs := a.ChangeSet
	lockData, err := state.New(a.CommitHash, a.ReleaseVersion, cs.AllFileHashes, cs.ComposerHash, cs.PackageHash, cs.GoModHash, cs.RequirementsHash).ToJSON()
	if err != nil {
		return err
	}
	if err := c.WriteRemoteBytes(d.remote("deploy.lock"), lockData); err != nil {
		// Non-fatal, but log it
		d.log.Error("Failed to upload deploy.lock: %v", err)
	}

	d.log.Info("Cleaning up old releases...")
	if err := c.CleanupOldReleases(releasesDir, d.env.ReleasesToKeep); err != nil {
		// Non-fatal
		d.log.Error("Failed to cleanup old releases: %v", err)
	}

	d.log.Success("Deployment to %s successful!", d.envName)
	return nil
}

// uploadRelease puts the artifact at finalDir: a delta over a hardlinked copy of the
// previous release when incremental_upload is on and possible, else the full archive.
// Content is staged in finalDir.staging and renamed into place.
func (d *Deployer) uploadRelease(c *ssh.Client, a *PrebuiltArtifact, previousLock *state.DeployLock, finalDir string) error {
	stagingDir := finalDir + ".staging"

	var hashes map[string]string
	uploaded := false
	if d.env.IncrementalUpload {
		var err error
		if hashes, err = artifact.HashTree(a.artifactDir); err != nil {
			return fmt.Errorf("failed to hash artifact: %w", err)
		}
		if previousLock != nil {
			if err := d.uploadDelta(c, a, previousLock.LastDeploy.ReleaseDir, hashes, stagingDir); err != nil {
				d.log.Warn("Incremental upload not possible, uploading full release: %v", err)
				c.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", stagingDir))
			} else {
				uploaded = true
			}
		}
	}

	if !uploaded {
		chunks, err := a.chunks(d.chunkSize(), d.log)
		if err != nil {
			return err
		}
		extracted, _ := fsutil.CalculateDirSize(a.artifactDir)
		if err := d.uploadArchive(c, chunks, extracted, stagingDir); err != nil {
			return err
		}
	}

	if hashes != nil {
		hashFile := stagingDir + "/" + artifact.HashFileName
		c.Remove(hashFile) // may be a hardlink into the previous release: never write through it
		if err := c.WriteRemoteBytes(hashFile, artifact.EncodeHashes(hashes)); err != nil {
			d.log.Warn("Failed to write %s (next deploy will upload in full): %v", artifact.HashFileName, err)
		}
	}

	if err := c.Rename(stagingDir, finalDir); err != nil {
		c.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", stagingDir))
		return fmt.Errorf("failed to finalize release: %w", err)
	}
	return nil
}

// uploadDelta builds stagingDir as a hardlinked copy (cp -al) of the previous release,
// removes stale/changed paths and extracts an archive holding only the changed files.
func (d *Deployer) uploadDelta(c *ssh.Client, a *PrebuiltArtifact, prevVersion string, hashes map[string]string, stagingDir string) error {
	prevDir := d.remote("releases", prevVersion)
	data, err := c.ReadRemoteBytes(prevDir+"/"+artifact.HashFileName, 256<<20)
	if err != nil {
		return fmt.Errorf("previous release has no %s: %w", artifact.HashFileName, err)
	}

	// Dependency dirs rebuilt in this artifact replace the previous copy entirely
	var replaced []string
	for _, p := range d.reusableDirs() {
		if _, err := os.Lstat(filepath.Join(a.artifactDir, filepath.FromSlash(p))); err == nil {
			replaced = append(replaced, p)
		}
	}
	upload, remove := artifact.Delta(artifact.DecodeHashes(data), hashes, replaced)
	d.log.Info("Incremental upload: %d of %d files changed, %d paths to remove", len(upload), len(hashes), len(remove))

	if _, err := c.ExecuteCommand(fmt.Sprintf("cp -al -- %q %q", prevDir, stagingDir)); err != nil {
		return fmt.Errorf("hardlink copy of previous release failed: %w", err)
	}

	if len(remove) > 0 {
		listPath := stagingDir + ".remove"
		if err := c.WriteRemoteBytes(listPath, []byte(strings.Join(remove, "\n")+"\n")); err != nil {
			return err
		}
		cmd := fmt.Sprintf(`cd %q && while IFS= read -r f; do rm -rf -- "$f"; done < %q; rm -f -- %q`, stagingDir, listPath, listPath)
		if _, err := c.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to remove stale files: %w", err)
		}
	}
	if len(upload) == 0 {
		return nil
	}

	keep := make(map[string]bool, len(upload))
	var extracted int64
	for _, p := range upload {
		keep[p] = true
		if fi, err := os.Lstat(filepath.Join(a.artifactDir, filepath.FromSlash(p))); err == nil {
			extracted += fi.Size()
		}
	}
	base := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.delta.tar.gz", a.ReleaseVersion, d.envName))
	chunks, err := artifact.NewGenerator(a.artifactDir, a.ReleaseVersion, a.CommitHash).CompressChunkedFiltered(base, d.chunkSize(), keep)
	defer func() {
		for _, p := range chunks {
			os.Remove(p)
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to compress delta: %w", err)
	}
	return d.uploadArchive(c, chunks, extracted, stagingDir)
}

// reusableDirs lists release-relative dependency paths (vendor, node_modules, venv, ...)
// that may be reused from the previous release instead of being shipped.
func (d *Deployer) reusableDirs() []string {
	var dirs []string
	add := func(root string, paths []string, always string) {
		if always != "" {
			paths = append(paths, always)
		}
		for _, p := range paths {
			dirs = append(dirs, filepath.ToSlash(filepath.Join("app", root, p)))
		}
	}
	b := d.env.Builds
	if b.PHP.Enabled {
		add(b.PHP.ProjectRoot, b.PHP.ReusablePaths, "vendor")
	}
	if b.Frontend.Enabled {
		add(b.Frontend.ProjectRoot, b.Frontend.ReusablePaths, "node_modules")
	}
	if b.Python.Enabled {
		add(b.Python.ProjectRoot, b.Python.ReusablePaths, b.Python.VenvPath)
	}
	return dirs
}

// uploadArchive uploads archive chunks to remote_path, reassembles them and extracts
// the archive into stagingDir.
func (d *Deployer) uploadArchive(c *ssh.Client, chunks []string, extractedSize int64, stagingDir string) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no archive chunks to upload")
	}
	var compressed int64
	for _, p := range chunks {
		if fi, err := os.Stat(p); err == nil {
			compressed += fi.Size()
		}
	}
	if err := d.checkDiskSpace(c, compressed, extractedSize); err != nil {
		return err
	}

	// Chunks are named <archive>.001, .002, ...
	remoteArchive := d.remote(strings.TrimSuffix(filepath.Base(chunks[0]), filepath.Ext(chunks[0])))
	d.log.Info("Uploading %d chunk(s), %d MB, with %d workers...", len(chunks), compressed>>20, d.env.UploadWorkers)
	if err := c.UploadFilesParallel(chunks, d.env.RemotePath, d.env.UploadWorkers); err != nil {
		return fmt.Errorf("parallel upload failed: %w", err)
	}

	d.log.Info("Reassembling artifact on server...")
	defer c.ExecuteCommand(fmt.Sprintf("rm -f -- %q %q.*", remoteArchive, remoteArchive))
	if _, err := c.ExecuteCommand(fmt.Sprintf("cat %q.* > %q && rm -f %q.*", remoteArchive, remoteArchive, remoteArchive)); err != nil {
		return fmt.Errorf("failed to reassemble artifact on server: %w", err)
	}
	return c.ExtractArchive(remoteArchive, stagingDir)
}

// checkDiskSpace fails when the remote filesystem can't hold the deploy's peak usage:
// chunks + reassembled archive (2x compressed), then archive + extracted tree; +20% margin.
func (d *Deployer) checkDiskSpace(c *ssh.Client, compressed, extracted int64) error {
	avail, err := c.AvailableDiskBytes(d.env.RemotePath)
	if err != nil {
		d.log.Warn("Could not check remote disk space (continuing): %v", err)
		return nil
	}
	need := max(2*compressed, compressed+extracted) * 6 / 5
	if avail >= need {
		d.log.Info("Disk space check passed: %d MB available, %d MB required", avail>>20, need>>20)
		return nil
	}

	hint := "Free up space on the remote server, or lower releases_to_keep."
	if freed := d.reclaimableBytes(c); freed > 0 {
		hint = fmt.Sprintf("Deleting the releases that cleanup would remove (releases_to_keep: %d) frees ~%d MB. Remove them manually from %s, or lower releases_to_keep.",
			d.env.ReleasesToKeep, freed>>20, d.remote("releases"))
	}
	return verserrors.New(verserrors.CodeUploadFailed,
		fmt.Sprintf("Insufficient disk space: need %d MB, have %d MB available", need>>20, avail>>20),
		hint, nil)
}

// reclaimableBytes estimates the space freed by deleting the releases the post-deploy
// cleanup removes. Kept releases are passed to du first so data hardlinked with them
// is not counted as freed.
func (d *Deployer) reclaimableBytes(c *ssh.Client) int64 {
	releases, err := c.ListReleases(d.remote("releases"))
	if err != nil {
		return 0
	}
	state.SortReleases(releases)     // newest first
	keep := d.env.ReleasesToKeep - 1 // the new release takes one slot
	if keep < 0 || keep >= len(releases) {
		return 0
	}
	paths := make([]string, len(releases))
	for i, r := range releases {
		paths[i] = d.remote("releases", r)
	}
	sizes, err := c.DirSizesKB(paths)
	if err != nil {
		return 0
	}
	var kb int64
	for _, p := range paths[keep:] {
		kb += sizes[p]
	}
	return kb * 1024
}

// ─── Multi-deploy support ──────────────────────────────────────────────────

// PrebuiltArtifact holds the result of a local build that can be deployed to
// multiple servers without repeating the build step. Call Cleanup() when done.
type PrebuiltArtifact struct {
	ReleaseVersion string
	CommitHash     string
	ChunkPaths     []string             // local /tmp/*.tar.gz.001, .002, … chunk files (created on first full upload)
	ChangeSet      *changeset.ChangeSet // used for dependency reuse and deploy.lock
	artifactDir    string               // owned by Cleanup
	tmpRepo        string               // owned by Cleanup
	mu             sync.Mutex           // guards ChunkPaths for concurrent DeployWithArtifact calls
}

// Cleanup removes all temporary directories and chunk files created during build.
func (a *PrebuiltArtifact) Cleanup() {
	if a.tmpRepo != "" {
		os.RemoveAll(a.tmpRepo)
	}
	os.RemoveAll(a.artifactDir)
	for _, p := range a.ChunkPaths {
		os.Remove(p)
	}
}

// chunks compresses the artifact on first use. Incremental deploys never need it.
func (a *PrebuiltArtifact) chunks(chunkSize int64, log *logger.Logger) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ChunkPaths != nil {
		return a.ChunkPaths, nil
	}
	log.Info("Compressing release into chunks...")
	base := filepath.Join(os.TempDir(), a.ReleaseVersion+".tar.gz")
	paths, err := artifact.NewGenerator(a.artifactDir, a.ReleaseVersion, a.CommitHash).CompressChunked(base, chunkSize)
	if err != nil {
		for _, p := range paths {
			os.Remove(p)
		}
		return nil, fmt.Errorf("failed to compress release: %w", err)
	}
	a.ChunkPaths = paths
	return paths, nil
}

// prepareRepo validates tools and repository, runs pre_deploy_local hooks, checks the
// working tree and clones it to a temp dir owned by the caller.
func (d *Deployer) prepareRepo() (tmpRepo, commitHash string, err error) {
	if err = d.validateLocalTools(); err != nil {
		return
	}
	if err = git.ValidateRepository(d.repoPath); err != nil {
		err = fmt.Errorf("repository validation failed: %w", err)
		return
	}

	// pre_deploy_local hooks abort on failure; skipped on dry runs (no side effects)
	if !d.dryRun {
		if err = d.executePreDeployLocal(); err != nil {
			return
		}
	}

	if !d.skipDirtyCheck {
		var clean bool
		if clean, err = git.IsClean(d.repoPath); err != nil {
			return
		}
		if !clean {
			err = verserrors.Wrap(fmt.Errorf("working directory has uncommitted changes (use --skip-dirty-check to bypass)"))
			return
		}
	} else {
		d.log.Warn("Skipping clean working directory check (--skip-dirty-check active)")
	}

	d.log.Info("Cloning repository to temporary directory...")
	if tmpRepo, err = git.Clone(d.repoPath, ""); err != nil {
		return
	}
	if commitHash, err = git.GetCurrentCommit(tmpRepo); err != nil {
		os.RemoveAll(tmpRepo)
		return "", "", err
	}
	d.log.Info("Commit: %s", commitHash[:8])
	return
}

// detectChanges hashes tmpRepo against previousLock (nil = everything changed).
func (d *Deployer) detectChanges(tmpRepo string, previousLock *state.DeployLock) (*changeset.ChangeSet, error) {
	return changeset.NewDetector(
		tmpRepo, d.env.Ignored, d.env.RouteFiles,
		d.env.Builds.PHP.ProjectRoot, d.env.Builds.Go.ProjectRoot,
		d.env.Builds.Frontend.ProjectRoot, d.env.Builds.Python.ProjectRoot,
		d.env.Builds.Python.RequirementsFile,
		previousLock,
	).Detect()
}

// buildFrom builds cs from tmpRepo into a new artifact dir (owned by the caller) and
// writes its manifest.
func (d *Deployer) buildFrom(tmpRepo, commitHash string, cs *changeset.ChangeSet) (artifactDir, releaseVersion string, err error) {
	releaseVersion = artifact.GenerateReleaseVersion()
	d.log.Info("Release version: %s", releaseVersion)

	d.log.Info("Building artifacts...")
	artifactDir = filepath.Join(os.TempDir(), "versadeploy-artifact-"+releaseVersion)
	if err = os.MkdirAll(artifactDir, 0775); err != nil {
		return "", "", err
	}
	buildResult, err := builder.NewBuilder(tmpRepo, artifactDir, d.env, cs, d.log).Build()
	if err != nil {
		os.RemoveAll(artifactDir)
		return "", "", verserrors.Wrap(err)
	}

	d.log.Debug("Generating manifest...")
	gen := artifact.NewGenerator(artifactDir, releaseVersion, commitHash)
	if err = gen.GenerateManifest(buildResult); err != nil {
		os.RemoveAll(artifactDir)
		return "", "", err
	}
	return artifactDir, releaseVersion, nil
}

// buildRelease runs the build phase shared by BuildArtifact() and the local deploy
// flow as a full build (all files treated as changed). It never touches SSH. On error,
// any temp directories already created are cleaned up before returning.
func (d *Deployer) buildRelease() (artifactDir, tmpRepo, releaseVersion, commitHash string, cs *changeset.ChangeSet, err error) {
	if tmpRepo, commitHash, err = d.prepareRepo(); err != nil {
		return
	}
	if cs, err = d.detectChanges(tmpRepo, nil); err == nil {
		cs.Force = true
		artifactDir, releaseVersion, err = d.buildFrom(tmpRepo, commitHash, cs)
	}
	if err != nil {
		os.RemoveAll(tmpRepo)
	}
	return
}

// BuildArtifact performs the local build phase (validation, clone, build) without
// connecting to any remote server. The returned artifact can be passed to
// DeployWithArtifact for each target server. The caller must call artifact.Cleanup()
// when all DeployWithArtifact calls are complete.
func (d *Deployer) BuildArtifact() (*PrebuiltArtifact, error) {
	artifactDir, tmpRepo, releaseVersion, commitHash, cs, err := d.buildRelease()
	if err != nil {
		return nil, err
	}
	return &PrebuiltArtifact{
		ReleaseVersion: releaseVersion,
		CommitHash:     commitHash,
		ChangeSet:      cs,
		artifactDir:    artifactDir,
		tmpRepo:        tmpRepo,
	}, nil
}

// DeployWithArtifact deploys a pre-built artifact to this deployer's configured
// remote server, skipping the build phase. The artifact must have been produced by
// BuildArtifact(). Safe to call concurrently on different Deployer instances.
func (d *Deployer) DeployWithArtifact(a *PrebuiltArtifact) (returnErr error) {
	startTime := time.Now()
	d.log.Info("Deploying %s to %s...", a.ReleaseVersion, d.envName)
	checkTimeout := d.deadline()

	if d.dryRun {
		d.log.Info("DRY RUN — would deploy release %s to %s", a.ReleaseVersion, d.envName)
		return nil
	}

	defer func() {
		d.sendNotification(a.ReleaseVersion, a.CommitHash, returnErr, time.Since(startTime))
	}()

	sshClient, unlock, err := d.connectAndLock()
	if err != nil {
		return err
	}
	defer unlock()

	previousLock, err := d.fetchPreviousLock(sshClient)
	if err != nil {
		return err
	}

	// Skip if server already has this exact commit (unless --force)
	if previousLock != nil && previousLock.LastDeploy.CommitHash == a.CommitHash && !d.force {
		d.log.Info("Server already at commit %s — skipping", a.CommitHash[:8])
		return nil
	}
	if err := d.checkGoTarget(sshClient); err != nil {
		return err
	}

	return d.ship(sshClient, a, previousLock, checkTimeout)
}

// rollback attempts to rollback to previous release
func (d *Deployer) rollback(sshClient *ssh.Client, previousLock *state.DeployLock) error {
	if previousLock == nil {
		return fmt.Errorf("no previous deployment to rollback to")
	}

	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	relativeTarget := filepath.ToSlash(filepath.Join("releases", previousLock.LastDeploy.ReleaseDir))

	return sshClient.CreateSymlink(relativeTarget, currentSymlink)
}

func (d *Deployer) runHook(sshClient *ssh.Client, finalDir, hook string, previousLock *state.DeployLock) error {
	hookTimeout := time.Duration(d.env.HookTimeout) * time.Second
	if hookTimeout <= 0 {
		hookTimeout = 300 * time.Second
	}

	appPath := filepath.ToSlash(filepath.Join(finalDir, "app"))
	wrappedHook := fmt.Sprintf("cd %s && %s", appPath, hook)

	d.log.Info("Executing: %s (in %s)", hook, appPath)
	output, err := sshClient.ExecuteCommandWithTimeout(wrappedHook, hookTimeout)
	if err != nil {
		d.log.Error("Hook failed: %s\nOutput: %s", hook, output)

		// Rollback on hook failure
		if previousLock != nil {
			d.log.Info("Critical Error in Hook: Deployment will be rolled back to version %s", previousLock.LastDeploy.ReleaseDir)
			if rollbackErr := d.rollback(sshClient, previousLock); rollbackErr != nil {
				return fmt.Errorf("hook failed and rollback also failed: %w", rollbackErr)
			}
			return fmt.Errorf("post-deploy hook failed (rolled back to %s): %w", previousLock.LastDeploy.ReleaseDir, err)
		}
		return fmt.Errorf("post-deploy hook failed (no previous version for rollback): %w", err)
	}

	d.log.Info("Hook output [%s]: %s", hook, strings.TrimSpace(output))
	return nil
}

func (d *Deployer) executePostDeployHooks(sshClient *ssh.Client, finalDir string, rollbackLock *state.DeployLock) error {
	if len(d.env.PostDeploy) == 0 {
		return nil
	}

	d.log.Info("Running post-deploy hooks...")

	for _, hookConfig := range d.env.PostDeploy {
		if hookConfig.Command != "" {
			if err := d.runHook(sshClient, finalDir, hookConfig.Command, rollbackLock); err != nil {
				return err
			}
		} else if len(hookConfig.Parallel) > 0 {
			var g errgroup.Group
			d.log.Info("Executing parallel hook group (%d commands)...", len(hookConfig.Parallel))
			for _, h := range hookConfig.Parallel {
				cmd := h // closure capture
				g.Go(func() error {
					return d.runHook(sshClient, finalDir, cmd, rollbackLock)
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	return nil
}

// executePreDeployLocal runs pre_deploy_local hooks locally; aborts deploy on failure.
func (d *Deployer) executePreDeployLocal() error {
	if len(d.env.PreDeployLocal) == 0 {
		return nil
	}

	d.log.Info("Running pre_deploy_local hooks...")
	for _, hookConfig := range d.env.PreDeployLocal {
		cmds := []string{}
		if hookConfig.Command != "" {
			cmds = []string{hookConfig.Command}
		} else {
			cmds = hookConfig.Parallel
		}
		for _, cmd := range cmds {
			d.log.Info("  Local: %s", cmd)
			var outBuf bytes.Buffer
			c := exec.Command("sh", "-c", cmd)
			c.Dir = d.repoPath
			c.Stdout = &outBuf
			c.Stderr = &outBuf
			if err := c.Run(); err != nil {
				d.log.Error("pre_deploy_local hook failed: %s\nOutput: %s", cmd, outBuf.String())
				return fmt.Errorf("pre_deploy_local hook failed: %w", err)
			}
			d.log.Info("  Output: %s", strings.TrimSpace(outBuf.String()))
		}
	}
	return nil
}

// executePreDeployServer runs pre_deploy_server hooks on the remote; never aborts deploy.
func (d *Deployer) executePreDeployServer(sshClient *ssh.Client, finalDir string) {
	if len(d.env.PreDeployServer) == 0 {
		return
	}

	d.log.Info("Running pre_deploy_server hooks (non-fatal)...")
	for _, hookConfig := range d.env.PreDeployServer {
		if hookConfig.Command != "" {
			if err := d.runHook(sshClient, finalDir, hookConfig.Command, nil); err != nil {
				d.log.Warn("pre_deploy_server hook failed (continuing): %v", err)
			}
		} else if len(hookConfig.Parallel) > 0 {
			var g errgroup.Group
			for _, h := range hookConfig.Parallel {
				cmd := h
				g.Go(func() error {
					return d.runHook(sshClient, finalDir, cmd, nil)
				})
			}
			if err := g.Wait(); err != nil {
				d.log.Warn("pre_deploy_server parallel hook failed (continuing): %v", err)
			}
		}
	}
}

// Rollback rolls back to the previous release
func (d *Deployer) Rollback() error {
	if d.env.Local {
		return errLocalUnsupported("rollback")
	}

	d.log.Info("Rolling back %s...", d.envName)

	// Connect to remote
	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	// Read current symlink
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	currentTarget, err := sshClient.ReadSymlink(currentSymlink)
	if err != nil {
		return fmt.Errorf("failed to read current symlink: %w", err)
	}

	d.log.Info("Current release: %s", filepath.Base(currentTarget))

	// List all releases
	releasesDir := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases"))
	releases, err := sshClient.ListReleases(releasesDir)
	if err != nil {
		return err
	}

	if len(releases) < 2 {
		return fmt.Errorf("no previous release to rollback to")
	}

	// Sort releases (newest first)
	state.SortReleases(releases)
	sorted := releases

	// Find previous (skip current if it's in the list)
	var previousRelease string
	currentRelease := filepath.Base(currentTarget)
	for _, release := range sorted {
		if release != currentRelease {
			previousRelease = release
			break
		}
	}

	if previousRelease == "" {
		return fmt.Errorf("could not determine previous release")
	}

	d.log.Info("Rolling back to: %s", previousRelease)

	// Switch symlink
	relativeTarget := filepath.ToSlash(filepath.Join("releases", previousRelease))
	if err := sshClient.CreateSymlink(relativeTarget, currentSymlink); err != nil {
		return err
	}

	d.log.Success("Rollback successful!")
	return nil
}

// Status shows deployment status
func (d *Deployer) Status() error {
	if d.env.Local {
		return errLocalUnsupported("status")
	}

	d.log.Info("Status for %s:", d.envName)

	// Connect to remote
	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	// Read current symlink
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	currentTarget, err := sshClient.ReadSymlink(currentSymlink)
	if err != nil {
		d.log.Info("No active deployment")
		return nil
	}

	d.log.Info("Current release: %s", filepath.Base(currentTarget))

	// List all releases
	releasesDir := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases"))
	releases, err := sshClient.ListReleases(releasesDir)
	if err != nil {
		return err
	}

	d.log.Info("Available releases: %d", len(releases))
	for _, release := range releases {
		marker := " "
		if release == filepath.Base(currentTarget) {
			marker = "→"
		}
		d.log.Info("  %s %s", marker, release)
	}

	return nil
}

// calculateDirectorySize calculates the total size of a directory
func (d *Deployer) calculateDirectorySize(dirPath string) (int64, error) {
	return fsutil.CalculateDirSize(dirPath)
}

// validateLocalTools checks if necessary build tools are available on the system
func (d *Deployer) validateLocalTools() error {
	var g errgroup.Group

	// Check PHP tools
	if d.env.Builds.PHP.Enabled {
		g.Go(func() error {
			cmd := "composer"
			if d.env.Builds.PHP.ComposerCommand != "" {
				parts := strings.Fields(d.env.Builds.PHP.ComposerCommand)
				if len(parts) > 0 {
					cmd = parts[0]
				}
			}
			if _, err := exec.LookPath(cmd); err != nil {
				return verserrors.New(verserrors.CodeBuildFailed,
					fmt.Sprintf("PHP build tool '%s' not found", cmd),
					fmt.Sprintf("Install %s or ensure it is in your PATH.", cmd), nil)
			}
			return nil
		})
	}

	// Check Go tools
	if d.env.Builds.Go.Enabled {
		g.Go(func() error {
			if _, err := exec.LookPath("go"); err != nil {
				return verserrors.New(verserrors.CodeBuildFailed,
					"Go compiler not found",
					"Install Go (https://golang.org/dl/) and ensure it is in your PATH.", nil)
			}
			return nil
		})
	}

	// Check Frontend tools
	if d.env.Builds.Frontend.Enabled {
		g.Go(func() error {
			tools := []string{}
			if d.env.Builds.Frontend.NPMCommand != "" {
				parts := strings.Fields(d.env.Builds.Frontend.NPMCommand)
				if len(parts) > 0 {
					tools = append(tools, parts[0])
				}
			}
			if d.env.Builds.Frontend.CompileCommand != "" {
				parts := strings.Fields(d.env.Builds.Frontend.CompileCommand)
				if len(parts) > 0 {
					cmd := parts[0]
					if !strings.HasPrefix(cmd, "./") && !strings.HasPrefix(cmd, ".\\") {
						tools = append(tools, cmd)
					}
				}
			}

			for _, tool := range tools {
				if _, err := exec.LookPath(tool); err != nil {
					return verserrors.New(verserrors.CodeBuildFailed,
						fmt.Sprintf("Frontend build tool '%s' not found", tool),
						fmt.Sprintf("Install %s (npm, pnpm, yarn, etc.) and ensure it is in your PATH.", tool), nil)
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// handleSharedPaths manages symbolic links for persistent directories
func (d *Deployer) handleSharedPaths(sshClient *ssh.Client, releaseDir string) error {
	if len(d.env.SharedPaths) == 0 {
		return nil
	}

	d.log.Info("Linking shared directories...")
	sharedBase := filepath.ToSlash(filepath.Join(d.env.RemotePath, "shared"))

	// Ensure shared directory exists via SFTP
	sshClient.MkdirAll(sharedBase)

	for _, path := range d.env.SharedPaths {
		// Clean the path to avoid directory traversal or trailing slashes
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if strings.HasPrefix(cleanPath, "../") || cleanPath == ".." {
			continue // Security: don't allow escaping release dir
		}

		// Path in release (now inside 'app' subfolder)
		releasePath := filepath.ToSlash(filepath.Join(releaseDir, "app", cleanPath))
		// Path in shared (e.g. shared/app/storage)
		sharedPath := filepath.ToSlash(filepath.Join(sharedBase, cleanPath))

		// 1. Ensure shared target exists via SFTP
		sshClient.MkdirAll(sharedPath)

		// 2. Remove directory in release if it exists to make room for symlink
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", releasePath))

		// 3. Create parent directory in release if needed via SFTP
		sshClient.MkdirAll(filepath.Dir(releasePath))

		// 4. Create symlink (use absolute path for shared target to be safe)
		// We use ln -sf directly for shared paths as they don't need the atomic switch logic of 'current'
		cmd := fmt.Sprintf("ln -sfn %q %q", sharedPath, releasePath)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to link shared path %s: %w", cleanPath, err)
		}
		d.log.Info("  Linked: %s -> %s", cleanPath, sharedPath)
	}

	return nil
}

// reuseDependencies attempts to recover vendor/node_modules and other build assets from previous release using hardlinks
func (d *Deployer) reuseDependencies(sshClient *ssh.Client, previousVersion, finalDir string, cs *changeset.ChangeSet) error {
	if previousVersion == "" {
		return nil
	}

	// Internal helper to reuse a specific path
	reusePath := func(projectRoot, relPath string) error {
		oldPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, "app", projectRoot, relPath))
		oldPathLegacy := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, projectRoot, relPath))
		newPath := filepath.ToSlash(filepath.Join(finalDir, "app", projectRoot, relPath))

		// Check if it's missing in new but exists in old (tries /app first, then legacy root)
		// Use SFTP for existence check as it's more reliable than shell [ -e ]
		sourceToUse := ""
		if exists, _ := sshClient.FileExists(oldPath); exists {
			sourceToUse = oldPath
		} else if exists, _ := sshClient.FileExists(oldPathLegacy); exists {
			sourceToUse = oldPathLegacy
		}

		if sourceToUse != "" {
			// Check if already exists in new artifact
			if exists, _ := sshClient.FileExists(newPath); !exists {
				if err := sshClient.MkdirAll(filepath.Dir(newPath)); err != nil {
					return fmt.Errorf("failed to create directory for reusable path %s: %w", relPath, err)
				}
				cmd := fmt.Sprintf("cp -al -- %q %q", sourceToUse, newPath)
				if _, err := sshClient.ExecuteCommand(cmd); err != nil {
					return fmt.Errorf("failed to reuse path %s from previous release: %w", relPath, err)
				}
				d.log.Info("  Reused: %s", newPath)
			}
		}

		return nil
	}

	// Reuse release-level path (outside app/), e.g. bin/app for Go
	reuseReleasePath := func(relPath string) error {
		oldPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, relPath))
		newPath := filepath.ToSlash(filepath.Join(finalDir, relPath))

		sourceToUse := ""
		if exists, _ := sshClient.FileExists(oldPath); exists {
			sourceToUse = oldPath
		}

		if sourceToUse == "" {
			return nil
		}

		if exists, _ := sshClient.FileExists(newPath); exists {
			return nil
		}

		if err := sshClient.MkdirAll(filepath.Dir(newPath)); err != nil {
			return fmt.Errorf("failed to create directory for reusable release path %s: %w", relPath, err)
		}

		cmd := fmt.Sprintf("cp -al -- %q %q", sourceToUse, newPath)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to reuse release path %s from previous release: %w", relPath, err)
		}

		d.log.Info("  Reused: %s", newPath)
		return nil
	}

	// PHP
	if d.env.Builds.PHP.Enabled && !cs.ComposerChanged {
		// Always include vendor if not explicitly in ReusablePaths
		paths := d.env.Builds.PHP.ReusablePaths
		hasVendor := false
		for _, p := range paths {
			if p == "vendor" {
				hasVendor = true
				break
			}
		}
		if !hasVendor {
			paths = append(paths, "vendor")
		}

		for _, p := range paths {
			if err := reusePath(d.env.Builds.PHP.ProjectRoot, p); err != nil {
				return err
			}
		}
	}

	// Frontend
	if d.env.Builds.Frontend.Enabled && !cs.PackageChanged {
		// Always include node_modules if not explicitly in ReusablePaths
		paths := d.env.Builds.Frontend.ReusablePaths
		hasNodeModules := false
		for _, p := range paths {
			if p == "node_modules" {
				hasNodeModules = true
				break
			}
		}
		if !hasNodeModules {
			paths = append(paths, "node_modules")
		}

		for _, p := range paths {
			if err := reusePath(d.env.Builds.Frontend.ProjectRoot, p); err != nil {
				return err
			}
		}
	}

	// Go
	if d.env.Builds.Go.Enabled && !cs.GoModChanged && len(cs.GoFiles) == 0 {
		goBinary := filepath.ToSlash(filepath.Join(d.env.Builds.Go.DeployPath, d.env.Builds.Go.BinaryName))
		if err := reuseReleasePath(goBinary); err != nil {
			return err
		}
	}

	// Python
	if d.env.Builds.Python.Enabled && !cs.RequirementsChanged {
		paths := d.env.Builds.Python.ReusablePaths
		hasVenv := false
		for _, p := range paths {
			if p == d.env.Builds.Python.VenvPath {
				hasVenv = true
				break
			}
		}
		if !hasVenv && d.env.Builds.Python.VenvPath != "" {
			paths = append(paths, d.env.Builds.Python.VenvPath)
		}

		if d.env.Builds.Python.WebServer {
			paths = append(paths, "run_server.sh")
			if d.env.Builds.Python.ServiceName != "" {
				paths = append(paths, d.env.Builds.Python.ServiceName+".service")
			}
		}

		if d.env.Builds.Python.BuildBinary && d.env.Builds.Python.BinaryName != "" {
			paths = append(paths, d.env.Builds.Python.BinaryName)
		}

		for _, p := range paths {
			if err := reusePath(d.env.Builds.Python.ProjectRoot, p); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Deployer) validateRuntimeArtifacts(sshClient *ssh.Client, finalDir string, cs *changeset.ChangeSet) error {
	if d.env.Builds.Go.Enabled {
		binPath := filepath.ToSlash(filepath.Join(finalDir, d.env.Builds.Go.DeployPath, d.env.Builds.Go.BinaryName))
		exists, err := sshClient.FileExists(binPath)
		if err != nil {
			return fmt.Errorf("failed to verify Go binary on release: %w", err)
		}
		if !exists {
			return verserrors.New(
				verserrors.CodeBuildFailed,
				fmt.Sprintf("release is missing required Go binary: %s", binPath),
				"Ensure Go build is enabled and/or reuse from previous release succeeds.",
				nil,
			)
		}
	}

	if d.env.Builds.Python.Enabled {
		projectRoot := d.env.Builds.Python.ProjectRoot
		appDir := filepath.ToSlash(filepath.Join(finalDir, "app", projectRoot))

		if d.env.Builds.Python.BuildBinary {
			binPath := filepath.ToSlash(filepath.Join(appDir, d.env.Builds.Python.BinaryName))
			exists, err := sshClient.FileExists(binPath)
			if err != nil {
				return fmt.Errorf("failed to verify Python binary on release: %w", err)
			}
			if !exists {
				return verserrors.New(
					verserrors.CodeBuildFailed,
					fmt.Sprintf("release is missing required Python binary: %s", binPath),
					"Set python.build_binary correctly and ensure PyInstaller build succeeds.",
					nil,
				)
			}
		}

		if d.env.Builds.Python.WebServer {
			scriptPath := filepath.ToSlash(filepath.Join(appDir, "run_server.sh"))
			exists, err := sshClient.FileExists(scriptPath)
			if err != nil {
				return fmt.Errorf("failed to verify Python run script on release: %w", err)
			}
			if !exists {
				return verserrors.New(
					verserrors.CodeBuildFailed,
					fmt.Sprintf("release is missing required Python run script: %s", scriptPath),
					"Enable python.web_server with a valid entry_point or run_command.",
					nil,
				)
			}
		}

		// If Python files changed but requirements did not, verify configured reusable runtime paths when present.
		if cs != nil && len(cs.PythonFiles) > 0 && !cs.RequirementsChanged {
			for _, reusable := range d.env.Builds.Python.ReusablePaths {
				reusablePath := filepath.ToSlash(filepath.Join(appDir, reusable))
				exists, err := sshClient.FileExists(reusablePath)
				if err != nil {
					return fmt.Errorf("failed to verify Python reusable path %s: %w", reusablePath, err)
				}
				if !exists {
					d.log.Warn("Python reusable path not found in release: %s", reusablePath)
				}
			}
		}
	}

	if d.env.Builds.PHP.Enabled {
		phpVendorPath := filepath.ToSlash(filepath.Join(finalDir, "app", d.env.Builds.PHP.ProjectRoot, "vendor"))
		exists, err := sshClient.FileExists(phpVendorPath)
		if err != nil {
			return fmt.Errorf("failed to verify PHP vendor path on release: %w", err)
		}
		if !exists {
			return verserrors.New(
				verserrors.CodeBuildFailed,
				fmt.Sprintf("release is missing required PHP dependencies: %s", phpVendorPath),
				"Ensure composer install ran successfully or vendor was reused from previous release.",
				nil,
			)
		}
	}

	return nil
}

// handlePreservedPaths restores files/directories from the previous release that should NOT be updated
func (d *Deployer) handlePreservedPaths(sshClient *ssh.Client, previousVersion, finalDir string) error {
	if len(d.env.PreservedPaths) == 0 || previousVersion == "" {
		return nil
	}

	d.log.Info("Restoring preserved paths (locking to server version)...")
	for _, path := range d.env.PreservedPaths {
		cleanPath := filepath.ToSlash(filepath.Clean(path))

		// Paths are inside 'app' in the new structure, but might be at root in legacy releases
		oldPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, "app", cleanPath))
		oldPathLegacy := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, cleanPath))
		newPath := filepath.ToSlash(filepath.Join(finalDir, "app", cleanPath))

		// Check if source exists before trying to copy (tries /app first, then legacy root)
		// Use SFTP instead of shell for better reliability
		sourceToUse := ""
		if exists, _ := sshClient.FileExists(oldPath); exists {
			sourceToUse = oldPath
		} else if exists, _ := sshClient.FileExists(oldPathLegacy); exists {
			sourceToUse = oldPathLegacy
			d.log.Info("  Found %s in legacy root (migrating to /app structure)", cleanPath)
		}

		if sourceToUse != "" {
			// Remove whatever came in the artifact to ensure a clean copy
			sshClient.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", newPath))

			// Copy from old to new (using -p to preserve attributes)
			// We still use shell for cp as it's the fastest way to copy on server
			cmd := fmt.Sprintf("cp -rfp -- %q %q", sourceToUse, newPath)
			if _, err := sshClient.ExecuteCommand(cmd); err != nil {
				return fmt.Errorf("failed to preserve path %s: %w", cleanPath, err)
			}
			d.log.Info("  Preserved: %s (restored from previous release)", cleanPath)
		} else {
			d.log.Warn("  Could not preserve %s: source not found in previous release (tried %s and %s)", cleanPath, oldPath, oldPathLegacy)
		}
	}

	return nil
}

// ReloadServices connects to the remote server and re-executes all services_reload commands.
func (d *Deployer) ReloadServices() error {
	if d.env.Local {
		return errLocalUnsupported("services-reload")
	}
	if len(d.env.ServicesReload) == 0 {
		d.log.Info("No services_reload commands configured")
		return nil
	}

	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	d.executeServicesReload(sshClient)
	d.log.Success("Services reloaded!")
	return nil
}

// executeServicesReload runs configured service reload commands after symlink switch.
// This is critical for clearing PHP-FPM OPcache/realpath_cache, reloading Apache/Nginx, etc.
// Failures are logged as warnings but do NOT trigger rollback (the symlink is already switched).
func (d *Deployer) executeServicesReload(sshClient *ssh.Client) {
	if len(d.env.ServicesReload) == 0 {
		return
	}

	d.log.Info("Reloading services...")
	reloadTimeout := 30 * time.Second

	for _, cmd := range d.env.ServicesReload {
		d.log.Info("  Executing: %s", cmd)
		output, err := sshClient.ExecuteCommandWithTimeout(cmd, reloadTimeout)
		if err != nil {
			d.log.Warn("  Service reload command failed (non-fatal): %s — %v", cmd, err)
			if output != "" {
				d.log.Warn("  Output: %s", strings.TrimSpace(output))
			}
		} else {
			if output != "" {
				d.log.Info("  Output: %s", strings.TrimSpace(output))
			}
			d.log.Info("  ✓ %s", cmd)
		}
	}
}

// performHealthCheck verifies the application is working after deployment, via an HTTP
// URL and/or a remote command run in the new release's app dir (exit 0 = healthy).
// If the health check fails after all retries, it rolls back to the previous release.
func (d *Deployer) performHealthCheck(previousLock *state.DeployLock, sshClient *ssh.Client, finalDir string) error {
	hc := d.env.HealthCheck
	if hc.URL == "" && hc.Command == "" {
		return nil
	}

	expectedStatus := hc.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	timeout := hc.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	retries := hc.Retries
	if retries <= 0 {
		retries = 3
	}
	retryDelay := hc.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2
	}

	d.log.Info("Running health check (%d retries)...", retries)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	probe := func() error {
		if hc.Command != "" {
			cmd := fmt.Sprintf("cd %q && %s", filepath.ToSlash(filepath.Join(finalDir, "app")), hc.Command)
			if out, err := sshClient.ExecuteCommandWithTimeout(cmd, time.Duration(timeout)*time.Second); err != nil {
				return fmt.Errorf("command %q failed: %w (output: %s)", hc.Command, err, strings.TrimSpace(out))
			}
		}
		if hc.URL != "" {
			resp, err := client.Get(hc.URL)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			resp.Body.Close()
			if resp.StatusCode != expectedStatus {
				return fmt.Errorf("expected status %d, got %d", expectedStatus, resp.StatusCode)
			}
		}
		return nil
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		if lastErr = probe(); lastErr == nil {
			d.log.Info("  ✓ Health check passed")
			return nil
		}
		lastErr = fmt.Errorf("attempt %d/%d: %w", attempt, retries, lastErr)
		d.log.Warn("  Health check %s", lastErr)
		if attempt < retries {
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}
	}

	d.log.Error("Health check failed after %d attempts", retries)

	// Rollback on health check failure
	if previousLock != nil {
		d.log.Info("Rolling back due to health check failure...")
		if err := d.rollback(sshClient, previousLock); err != nil {
			return fmt.Errorf("health check failed and rollback also failed: %w (health: %v)", err, lastErr)
		}
		// Re-reload services after rollback
		d.executeServicesReload(sshClient)
		return fmt.Errorf("health check failed (rolled back to %s): %w", previousLock.LastDeploy.ReleaseDir, lastErr)
	}

	return fmt.Errorf("health check failed (no previous version for rollback): %w", lastErr)
}

// sendNotification sends a webhook notification about the deployment result.
func (d *Deployer) sendNotification(releaseVersion, commit string, deployErr error, duration time.Duration) {
	if d.env.Notifications.WebhookURL == "" {
		return
	}

	isSuccess := deployErr == nil
	if isSuccess && !d.env.Notifications.OnSuccess {
		return
	}
	if !isSuccess && !d.env.Notifications.OnFailure {
		return
	}

	status := "success"
	errorMsg := ""
	if !isSuccess {
		status = "failure"
		errorMsg = deployErr.Error()
	}

	// "text" is rendered by Slack and Teams webhooks, "content" by Discord.
	text := fmt.Sprintf("[%s] %s → %s: %s (release %s, %.0fs)", strings.ToUpper(status), d.cfg.Project, d.envName, status, releaseVersion, duration.Seconds())
	if errorMsg != "" {
		text += "\n" + errorMsg
	}

	payload := map[string]interface{}{
		"text":        text,
		"content":     text,
		"project":     d.cfg.Project,
		"environment": d.envName,
		"release":     releaseVersion,
		"commit":      commit,
		"status":      status,
		"error":       errorMsg,
		"duration_s":  duration.Seconds(),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		d.log.Warn("Failed to marshal notification payload: %v", err)
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(d.env.Notifications.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		d.log.Warn("Failed to send deployment notification: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.log.Info("Deployment notification sent (%s)", status)
	} else {
		d.log.Warn("Deployment notification returned status %d", resp.StatusCode)
	}
}

// RollbackTo rolls back to a specific release version
func (d *Deployer) RollbackTo(targetVersion string) error {
	if d.env.Local {
		return errLocalUnsupported("rollback")
	}

	d.log.Info("Rolling back %s to version %s...", d.envName, targetVersion)

	// Connect to remote
	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	// Validate the target release exists
	releasesDir := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases"))
	releases, err := sshClient.ListReleases(releasesDir)
	if err != nil {
		return err
	}

	found := false
	for _, r := range releases {
		if r == targetVersion {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("release %s not found on server (available: %s)", targetVersion, strings.Join(releases, ", "))
	}

	// Switch symlink
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	absoluteTarget := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", targetVersion))
	if err := sshClient.CreateSymlink(absoluteTarget, currentSymlink); err != nil {
		return err
	}

	// Reload services after rollback
	d.executeServicesReload(sshClient)

	d.log.Success("Rollback to %s successful!", targetVersion)
	return nil
}

// RunHooks executes specific hooks against the currently active release.
// If indices is nil or empty, all post_deploy hooks are executed.
func (d *Deployer) RunHooks(indices []int) error {
	if d.env.Local {
		return errLocalUnsupported("hooks")
	}

	d.log.Info("Re-executing hooks on %s...", d.envName)

	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	// Find the active release directory
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	currentTarget, err := sshClient.ReadSymlink(currentSymlink)
	if err != nil {
		return fmt.Errorf("failed to read current symlink: %w", err)
	}

	d.log.Info("Active release: %s", filepath.Base(currentTarget))

	// Resolve absolute path — currentTarget may be relative (releases/xxx)
	var finalDir string
	if strings.HasPrefix(currentTarget, "/") {
		finalDir = currentTarget
	} else {
		finalDir = filepath.ToSlash(filepath.Join(d.env.RemotePath, currentTarget))
	}

	hooks := d.env.PostDeploy
	if len(hooks) == 0 {
		d.log.Info("No post_deploy hooks configured")
		return nil
	}

	// If specific indices are provided, filter hooks
	if len(indices) > 0 {
		var selected []config.HookConfig
		for _, idx := range indices {
			if idx < 0 || idx >= len(hooks) {
				return fmt.Errorf("hook index %d out of range (0-%d)", idx, len(hooks)-1)
			}
			selected = append(selected, hooks[idx])
		}
		hooks = selected
	}

	hookTimeout := time.Duration(d.env.HookTimeout) * time.Second
	if hookTimeout <= 0 {
		hookTimeout = 300 * time.Second
	}

	for _, hookConfig := range hooks {
		if hookConfig.Command != "" {
			appPath := filepath.ToSlash(filepath.Join(finalDir, "app"))
			wrappedHook := fmt.Sprintf("cd %s && %s", appPath, hookConfig.Command)
			d.log.Info("Executing: %s", hookConfig.Command)
			output, err := sshClient.ExecuteCommandWithTimeout(wrappedHook, hookTimeout)
			if err != nil {
				d.log.Error("Hook failed: %s — %v", hookConfig.Command, err)
				if output != "" {
					d.log.Error("Output: %s", strings.TrimSpace(output))
				}
				return fmt.Errorf("hook failed: %w", err)
			}
			if output != "" {
				d.log.Info("Output: %s", strings.TrimSpace(output))
			}
		} else if len(hookConfig.Parallel) > 0 {
			var g errgroup.Group
			d.log.Info("Executing parallel hook group (%d commands)...", len(hookConfig.Parallel))
			for _, h := range hookConfig.Parallel {
				cmd := h
				appPath := filepath.ToSlash(filepath.Join(finalDir, "app"))
				g.Go(func() error {
					wrappedHook := fmt.Sprintf("cd %s && %s", appPath, cmd)
					d.log.Info("Executing: %s", cmd)
					output, hookErr := sshClient.ExecuteCommandWithTimeout(wrappedHook, hookTimeout)
					if hookErr != nil {
						return fmt.Errorf("hook %q failed: %w", cmd, hookErr)
					}
					if output != "" {
						d.log.Info("Output [%s]: %s", cmd, strings.TrimSpace(output))
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	d.log.Success("Hooks executed successfully!")
	return nil
}

// ExecRemoteCommand executes an arbitrary command on the remote server
func (d *Deployer) ExecRemoteCommand(command string) (string, error) {
	if d.env.Local {
		return "", errLocalUnsupported("exec")
	}

	sshClient, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return "", verserrors.Wrap(err)
	}
	defer sshClient.Close()

	timeout := time.Duration(d.env.HookTimeout) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	output, err := sshClient.ExecuteCommandWithTimeout(command, timeout)
	if err != nil {
		return output, err
	}
	return output, nil
}
