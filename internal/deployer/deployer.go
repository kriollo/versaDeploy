package deployer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/versaDeploy/internal/artifact"
	"github.com/user/versaDeploy/internal/builder"
	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/git"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/state"
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
	log           *logger.Logger
}

// NewDeployer creates a new deployer
func NewDeployer(cfg *config.Config, envName, repoPath string, dryRun, initialDeploy bool, log *logger.Logger) (*Deployer, error) {
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
		log:           log,
	}, nil
}

// Deploy executes the full deployment workflow
func (d *Deployer) Deploy() error {
	d.log.Info("Starting deployment to %s", d.envName)

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
	detector := changeset.NewDetector(tmpRepo, d.env.Ignored, d.env.RouteFiles, previousLock)
	cs, err := detector.Detect()
	if err != nil {
		return err
	}

	if !cs.HasChanges() {
		d.log.Info("No changes detected - skipping deployment")
		return nil
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
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
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
	if _, err := sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p %s", releasesDir)); err != nil {
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

	// Upload to staging
	if err := sshClient.UploadDirectory(artifactDir, stagingDir); err != nil {
		return err
	}

	// Move staging to final (atomic)
	if _, err := sshClient.ExecuteCommand(fmt.Sprintf("mv %s %s", stagingDir, finalDir)); err != nil {
		// Cleanup staging on failure
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf %s", stagingDir))
		return fmt.Errorf("failed to finalize release: %w", err)
	}

	// Step 12: Atomic symlink switch
	d.log.Info("Activating release...")
	currentSymlink := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current"))
	relativeTarget := filepath.ToSlash(filepath.Join("releases", releaseVersion))

	if err := sshClient.CreateSymlink(relativeTarget, currentSymlink); err != nil {
		return err
	}

	// Step 13: Execute post-deploy hooks
	if len(d.env.PostDeploy) > 0 {
		d.log.Info("Running post-deploy hooks...")
		for _, hook := range d.env.PostDeploy {
			d.log.Info("Executing: %s", hook)
			output, err := sshClient.ExecuteCommand(hook)
			if err != nil {
				d.log.Error("Post-deploy hook failed: %s", output)
				// Rollback on hook failure
				d.log.Info("Attempting automatic rollback...")
				if rollbackErr := d.rollback(sshClient, previousLock); rollbackErr != nil {
					return fmt.Errorf("hook failed and rollback also failed: %w", rollbackErr)
				}
				return fmt.Errorf("post-deploy hook failed (rolled back): %w", err)
			}
			d.log.Info("Hook output: %s", output)
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
	os.MkdirAll(tmpUploadDir, 0755)
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

	// Find previous release (second newest)
	// Sort releases
	sorted := make([]string, len(releases))
	copy(sorted, releases)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] < sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

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
