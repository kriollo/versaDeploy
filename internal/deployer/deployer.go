package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/versaDeploy/internal/artifact"
	"github.com/user/versaDeploy/internal/builder"
	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/git"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/state"
	"golang.org/x/sync/errgroup"
)

const ReleasesToKeep = 5

// Deployer orchestrates the entire deployment process
type Deployer struct {
	cfg           *config.Config
	env           *config.Environment
	envName       string
	repoPath      string
	dryRun        bool
	initialDeploy bool
	force         bool
	log           *logger.Logger
}

// NewDeployer creates a new deployer
func NewDeployer(cfg *config.Config, envName, repoPath string, dryRun, initialDeploy, force bool, log *logger.Logger) (*Deployer, error) {
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	return &Deployer{
		cfg:           cfg,
		env:           env,
		envName:       envName,
		repoPath:      repoPath,
		dryRun:        dryRun,
		initialDeploy: initialDeploy,
		force:         force,
		log:           log,
	}, nil
}

// Deploy executes the full deployment workflow
func (d *Deployer) Deploy() error {
	d.log.Info("Starting deployment to %s", d.envName)

	// Step 0: Validate local tools
	if err := d.validateLocalTools(); err != nil {
		return err
	}

	// Step 1: Validate repository
	if err := git.ValidateRepository(d.repoPath); err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// Step 2: Check if working directory is clean
	clean, err := git.IsClean(d.repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return verserrors.Wrap(fmt.Errorf("working directory has uncommitted changes"))
	}

	// Step 3: Clone repository to clean temp directory
	d.log.Info("Cloning repository to temporary directory...")
	tmpRepo, err := git.Clone(d.repoPath, "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRepo)

	// Step 4: Get commit hash
	commitHash, err := git.GetCurrentCommit(tmpRepo)
	if err != nil {
		return err
	}
	d.log.Info("Commit: %s", commitHash[:8])

	// Step 5: Connect to remote server
	d.log.Info("Connecting to %s@%s...", d.env.SSH.User, d.env.SSH.Host)
	sshClient, err := ssh.NewClient(&d.env.SSH)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer sshClient.Close()

	// Step 5.5: Acquire deployment lock to prevent concurrent deployments
	lockDirPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, ".versa.lock"))
	d.log.Info("Acquiring deployment lock...")
	if err := sshClient.AcquireLock(lockDirPath); err != nil {
		return err
	}
	defer func() {
		d.log.Info("Releasing deployment lock...")
		if err := sshClient.ReleaseLock(lockDirPath); err != nil {
			d.log.Warn("Failed to release deployment lock: %v", err)
		}
	}()

	// Step 6: Fetch deploy.lock from remote
	lockPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "deploy.lock"))
	var previousLock *state.DeployLock

	exists, err := sshClient.FileExists(lockPath)
	if err != nil {
		return fmt.Errorf("failed to check deploy.lock: %w", err)
	}

	if exists {
		d.log.Info("Fetching deploy.lock from remote...")
		tmpLockFile := filepath.Join(os.TempDir(), "deploy.lock")
		if err := sshClient.DownloadFile(lockPath, tmpLockFile); err != nil {
			return err
		}
		defer os.Remove(tmpLockFile)

		lockData, err := os.ReadFile(tmpLockFile)
		if err != nil {
			return err
		}

		previousLock, err = state.Parse(lockData)
		if err != nil {
			return fmt.Errorf("failed to parse deploy.lock: %w", err)
		}
	} else {
		if !d.initialDeploy {
			return verserrors.Wrap(fmt.Errorf("deploy.lock not found on remote server"))
		}
		d.log.Info("First deployment detected (--initial-deploy)")
	}

	// Step 7: Calculate changeset
	d.log.Info("Calculating changes...")
	detector := changeset.NewDetector(tmpRepo, d.env.Ignored, d.env.RouteFiles, d.env.Builds.PHP.ProjectRoot, d.env.Builds.Go.ProjectRoot, d.env.Builds.Frontend.ProjectRoot, previousLock)
	cs, err := detector.Detect()
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

	d.log.Info("Changes detected: %d PHP, %d Twig, %d Go, %d Frontend files",
		len(cs.PHPFiles), len(cs.TwigFiles), len(cs.GoFiles), len(cs.FrontendFiles))

	if d.dryRun {
		d.log.Info("DRY RUN - would deploy these changes")
		return nil
	}

	// Step 8: Generate release version
	releaseVersion := artifact.GenerateReleaseVersion()
	d.log.Info("Release version: %s", releaseVersion)

	// Step 9: Build artifacts
	d.log.Info("Building artifacts...")
	artifactDir := filepath.Join(os.TempDir(), "versadeploy-artifact-"+releaseVersion)
	if err := os.MkdirAll(artifactDir, 0775); err != nil {
		return err
	}
	defer os.RemoveAll(artifactDir)

	builder := builder.NewBuilder(tmpRepo, artifactDir, d.env, cs)
	buildResult, err := builder.Build()
	if err != nil {
		return verserrors.Wrap(err)
	}

	// Step 10: Generate manifest
	d.log.Info("Generating manifest...")
	gen := artifact.NewGenerator(artifactDir, releaseVersion, commitHash)
	if err := gen.GenerateManifest(buildResult); err != nil {
		return err
	}

	if err := gen.Validate(); err != nil {
		return err
	}

	// Step 11: Upload artifact
	d.log.Info("Uploading artifact to remote server...")
	releasesDir := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases"))
	stagingDir := filepath.ToSlash(filepath.Join(releasesDir, releaseVersion+".staging"))
	finalDir := filepath.ToSlash(filepath.Join(releasesDir, releaseVersion))

	// Create releases directory if doesn't exist
	if _, err := sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p -- %q", releasesDir)); err != nil {
		return err
	}

	// Check disk space before upload
	artifactSize, err := d.calculateDirectorySize(artifactDir)
	if err != nil {
		d.log.Warn("Could not calculate artifact size: %v", err)
	} else {
		d.log.Info("Artifact size: %d MB", artifactSize/(1024*1024))
		if err := sshClient.CheckDiskSpace(releasesDir, artifactSize); err != nil {
			return verserrors.Wrap(err)
		}
	}

	// Step 10: Compress and upload to staging (Chunked Parallel)
	archiveName := fmt.Sprintf("%s.tar.gz", releaseVersion)
	localArchiveBase := filepath.Join(os.TempDir(), archiveName)
	remoteArchive := filepath.ToSlash(filepath.Join(d.env.RemotePath, archiveName))

	g := artifact.NewGenerator(artifactDir, releaseVersion, commitHash)
	d.log.Info("Compressing release into chunks...")

	// Use 10MB chunks for parallel upload optimization
	const chunkSize = 10 * 1024 * 1024
	chunkPaths, err := g.CompressChunked(localArchiveBase, chunkSize)
	if err != nil {
		return fmt.Errorf("failed to compress release: %w", err)
	}
	defer func() {
		for _, p := range chunkPaths {
			os.Remove(p)
		}
	}()

	d.log.Info("Uploading %d chunks in parallel to remote server...", len(chunkPaths))
	if err := sshClient.UploadFilesParallel(chunkPaths, d.env.RemotePath, 4); err != nil {
		return fmt.Errorf("parallel upload failed: %w", err)
	}

	// Reassemble chunks on the remote server
	d.log.Info("Reassembling artifact on server...")
	reassembleCmd := fmt.Sprintf("cat %q.* > %q && rm -f %q.*", remoteArchive, remoteArchive, remoteArchive)
	if _, err := sshClient.ExecuteCommand(reassembleCmd); err != nil {
		return fmt.Errorf("failed to reassemble artifact on server: %w", err)
	}

	// Extract on remote
	if err := sshClient.ExtractArchive(remoteArchive, stagingDir); err != nil {
		sshClient.ExecuteCommand(fmt.Sprintf("rm -f %s", remoteArchive))
		return err
	}

	// Cleanup remote archive
	sshClient.ExecuteCommand(fmt.Sprintf("rm -f -- %q", remoteArchive))

	if _, err := sshClient.ExecuteCommand(fmt.Sprintf("mv -T -- %q %q", stagingDir, finalDir)); err != nil {
		// Cleanup staging on failure
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", stagingDir))
		return fmt.Errorf("failed to finalize release: %w", err)
	}

	// Step 11.5: Handle shared paths
	if err := d.handleSharedPaths(sshClient, finalDir); err != nil {
		return err
	}

	// Step 11.6: Reuse dependencies from previous release if possible
	if previousLock != nil {
		d.reuseDependencies(sshClient, previousLock.LastDeploy.ReleaseDir, finalDir, cs)

		// Step 11.7: Restore preserved paths (files that should not be updated)
		if err := d.handlePreservedPaths(sshClient, previousLock.LastDeploy.ReleaseDir, finalDir); err != nil {
			return err
		}
	}

	// Step 12: Atomic symlink switch
	d.log.Info("Activating release...")
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	// Use absolute path for target to be more robust
	absoluteTarget := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", releaseVersion))

	d.log.Info("  Linking: %s -> %s", currentSymlink, absoluteTarget)

	if err := sshClient.CreateSymlink(absoluteTarget, currentSymlink); err != nil {
		return err
	}

	// Step 13: Execute post-deploy hooks
	if len(d.env.PostDeploy) > 0 {
		d.log.Info("Running post-deploy hooks...")
		hookTimeout := time.Duration(d.env.HookTimeout) * time.Second
		if hookTimeout <= 0 {
			hookTimeout = 300 * time.Second // Default 5 minutes
		}

		for _, hookConfig := range d.env.PostDeploy {
			if hookConfig.Command != "" {
				if err := d.runHook(sshClient, finalDir, hookConfig.Command, previousLock); err != nil {
					return err
				}
			} else if len(hookConfig.Parallel) > 0 {
				var g errgroup.Group
				d.log.Info("Executing parallel hook group (%d commands)...", len(hookConfig.Parallel))
				for _, h := range hookConfig.Parallel {
					cmd := h // closure capture
					g.Go(func() error {
						return d.runHook(sshClient, finalDir, cmd, previousLock)
					})
				}
				if err := g.Wait(); err != nil {
					return err // runHook already handles rollback and specific logging
				}
			}
		}
	}

	// Step 14: Update deploy.lock
	d.log.Info("Updating deploy.lock...")
	newLock := state.New(commitHash, releaseVersion, cs.AllFileHashes, cs.ComposerHash, cs.PackageHash, cs.GoModHash)
	lockData, err := newLock.ToJSON()
	if err != nil {
		return err
	}

	tmpLockFile := filepath.Join(os.TempDir(), "deploy.lock.new")
	if err := os.WriteFile(tmpLockFile, lockData, 0644); err != nil {
		return err
	}
	defer os.Remove(tmpLockFile)

	// Upload deploy.lock directly as a file
	tmpUploadDir := filepath.Join(os.TempDir(), "lockupload")
	os.MkdirAll(tmpUploadDir, 0775)
	defer os.RemoveAll(tmpUploadDir)

	lockUploadPath := filepath.Join(tmpUploadDir, "deploy.lock")
	if err := os.WriteFile(lockUploadPath, lockData, 0644); err != nil {
		return err
	}

	if err := sshClient.UploadDirectory(tmpUploadDir, d.env.RemotePath); err != nil {
		// Non-fatal, but log it
		d.log.Error("Failed to upload deploy.lock: %v", err)
	}

	// Step 15: Cleanup old releases
	d.log.Info("Cleaning up old releases...")
	if err := sshClient.CleanupOldReleases(releasesDir, ReleasesToKeep); err != nil {
		// Non-fatal
		d.log.Error("Failed to cleanup old releases: %v", err)
	}

	d.log.Success("Deployment successful!")
	return nil
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

// Rollback rolls back to the previous release
func (d *Deployer) Rollback() error {
	d.log.Info("Rolling back %s...", d.envName)

	// Connect to remote
	sshClient, err := ssh.NewClient(&d.env.SSH)
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
	d.log.Info("Status for %s:", d.envName)

	// Connect to remote
	sshClient, err := ssh.NewClient(&d.env.SSH)
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
	var size int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
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

	// Ensure shared directory exists
	sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p -- %q", sharedBase))

	for _, path := range d.env.SharedPaths {
		// Clean the path to avoid directory traversal or trailing slashes
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if strings.HasPrefix(cleanPath, "../") || cleanPath == ".." {
			continue // Security: don't allow escaping release dir
		}

		// Path in release (e.g. app/storage)
		releasePath := filepath.ToSlash(filepath.Join(releaseDir, cleanPath))
		// Path in shared (e.g. shared/app/storage)
		sharedPath := filepath.ToSlash(filepath.Join(sharedBase, cleanPath))

		// 1. Ensure shared target exists
		sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p -- %q", sharedPath))

		// 2. Remove directory in release if it exists to make room for symlink
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", releasePath))

		// 3. Create parent directory in release if needed
		sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p -- %q", filepath.Dir(releasePath)))

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
func (d *Deployer) reuseDependencies(sshClient *ssh.Client, previousVersion, finalDir string, cs *changeset.ChangeSet) {
	if previousVersion == "" {
		return
	}

	// Internal helper to reuse a specific path
	reusePath := func(projectRoot, relPath string) {
		oldPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, "app", projectRoot, relPath))
		newPath := filepath.ToSlash(filepath.Join(finalDir, "app", projectRoot, relPath))

		// Check if it's missing in new but exists in old
		// Use cp -al for cross-release hardlinking (fast and space efficient)
		cmd := fmt.Sprintf("if [ ! -e %q ] && [ -e %q ]; then mkdir -p -- %q && cp -al -- %q %q; fi",
			newPath, oldPath, filepath.Dir(newPath), oldPath, newPath)
		sshClient.ExecuteCommand(cmd)
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
			reusePath(d.env.Builds.PHP.ProjectRoot, p)
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
			reusePath(d.env.Builds.Frontend.ProjectRoot, p)
		}
	}
}

// handlePreservedPaths restores files/directories from the previous release that should NOT be updated
func (d *Deployer) handlePreservedPaths(sshClient *ssh.Client, previousVersion, finalDir string) error {
	if len(d.env.PreservedPaths) == 0 || previousVersion == "" {
		return nil
	}

	d.log.Info("Restoring preserved paths (locking to server version)...")
	for _, path := range d.env.PreservedPaths {
		cleanPath := filepath.ToSlash(filepath.Clean(path))

		// Paths are inside 'app' in both releases
		oldPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "releases", previousVersion, "app", cleanPath))
		newPath := filepath.ToSlash(filepath.Join(finalDir, "app", cleanPath))

		// Check if source exists before trying to copy
		// Use %q for safe quoting and direct shell return code check
		exists, err := sshClient.ExecuteCommand(fmt.Sprintf("if [ -e %q ]; then echo 'exists'; fi", oldPath))
		if err == nil && strings.TrimSpace(exists) == "exists" {
			// Remove whatever came in the artifact to ensure a clean copy
			sshClient.ExecuteCommand(fmt.Sprintf("rm -rf -- %q", newPath))

			// Copy from old to new (using -p to preserve attributes)
			cmd := fmt.Sprintf("cp -rfp -- %q %q", oldPath, newPath)
			if _, err := sshClient.ExecuteCommand(cmd); err != nil {
				return fmt.Errorf("failed to preserve path %s: %w", cleanPath, err)
			}
			d.log.Info("  Preserved: %s (restored from previous release)", cleanPath)
		} else {
			d.log.Warn("  Could not preserve %s: source not found in previous release (%s)", cleanPath, oldPath)
		}
	}

	return nil
}
