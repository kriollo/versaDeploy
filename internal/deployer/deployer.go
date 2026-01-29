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

	// Step 10: Compress and upload to staging
	d.log.Info("Compressing and uploading release...")
	archiveName := fmt.Sprintf("%s.tar.gz", releaseVersion)
	localArchive := filepath.Join(os.TempDir(), archiveName)
	remoteArchive := filepath.ToSlash(filepath.Join(d.env.RemotePath, archiveName))

	g := artifact.NewGenerator(artifactDir, releaseVersion, commitHash)
	if err := g.Compress(localArchive); err != nil {
		return fmt.Errorf("failed to compress release: %w", err)
	}
	defer os.Remove(localArchive)

	if err := sshClient.UploadFileWithProgress(localArchive, remoteArchive); err != nil {
		return err
	}

	// Extract on remote
	if err := sshClient.ExtractArchive(remoteArchive, stagingDir); err != nil {
		sshClient.ExecuteCommand(fmt.Sprintf("rm -f %s", remoteArchive))
		return err
	}

	// Cleanup remote archive
	sshClient.ExecuteCommand(fmt.Sprintf("rm -f %s", remoteArchive))

	if _, err := sshClient.ExecuteCommand(fmt.Sprintf("mv %s %s", stagingDir, finalDir)); err != nil {
		// Cleanup staging on failure
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf %s", stagingDir))
		return fmt.Errorf("failed to finalize release: %w", err)
	}

	// Step 11.5: Handle shared paths
	if err := d.handleSharedPaths(sshClient, finalDir); err != nil {
		return err
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
		hookTimeout := time.Duration(d.env.HookTimeout) * time.Second
		if hookTimeout <= 0 {
			hookTimeout = 300 * time.Second // Default 5 minutes
		}

		for _, hook := range d.env.PostDeploy {
			// Wrap the hook to run within the newly deployed 'app' directory
			appPath := filepath.ToSlash(filepath.Join(d.env.RemotePath, "current", "app"))
			wrappedHook := fmt.Sprintf("cd %s && %s", appPath, hook)

			d.log.Info("Executing: %s (in %s)", hook, appPath)
			output, err := sshClient.ExecuteCommandWithTimeout(wrappedHook, hookTimeout)
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

// validateLocalTools checks if necessary build tools are available on the system
func (d *Deployer) validateLocalTools() error {
	// Check PHP tools
	if d.env.Builds.PHP.Enabled {
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
				fmt.Sprintf("Install %s or ensure it is in your PATH. If you use a custom command, check your deploy.yml.", cmd), nil)
		}
	}

	// Check Go tools
	if d.env.Builds.Go.Enabled {
		if _, err := exec.LookPath("go"); err != nil {
			return verserrors.New(verserrors.CodeBuildFailed,
				"Go compiler not found",
				"Install Go (https://golang.org/dl/) and ensure it is in your PATH.", nil)
		}
	}

	// Check Frontend tools
	if d.env.Builds.Frontend.Enabled {
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
				// Don't check placeholders or relative scripts starting with ./
				// unless we want to be very strict.
				// For now let's check standard tools.
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
	}

	return nil
}

// handleSharedPaths manages symbolic links for persistent directories
func (d *Deployer) handleSharedPaths(sshClient *ssh.Client, releaseDir string) error {
	if len(d.env.SharedPaths) == 0 {
		return nil
	}

	d.log.Info("Linking shared directories...")
	sharedBase := filepath.ToSlash(filepath.Join(d.env.RemotePath, "shared"))

	// Ensure shared directory exists
	sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p %s", sharedBase))

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
		sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p %s", sharedPath))

		// 2. Remove directory in release if it exists to make room for symlink
		sshClient.ExecuteCommand(fmt.Sprintf("rm -rf %s", releasePath))

		// 3. Create parent directory in release if needed
		sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p %s", filepath.Dir(releasePath)))

		// 4. Create symlink (use absolute path for shared target to be safe)
		// We use ln -sf directly for shared paths as they don't need the atomic switch logic of 'current'
		cmd := fmt.Sprintf("ln -sfn %s %s", sharedPath, releasePath)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to link shared path %s: %w", cleanPath, err)
		}
		d.log.Info("  Linked: %s -> %s", cleanPath, sharedPath)
	}

	return nil
}
