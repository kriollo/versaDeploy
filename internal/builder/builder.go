package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
)

// BuildResult tracks what was built
type BuildResult struct {
	PHPFilesChanged      int
	GoBinaryRebuilt      bool
	FrontendCompiled     int
	ComposerUpdated      bool
	NPMUpdated           bool
	TwigCacheCleanup     bool
	RouteCacheRegenerate bool
}

// Builder orchestrates all build operations
type Builder struct {
	repoPath    string
	artifactDir string
	config      *config.Environment
	changeset   *changeset.ChangeSet
	result      *BuildResult
}

// NewBuilder creates a new builder
func NewBuilder(repoPath, artifactDir string, cfg *config.Environment, cs *changeset.ChangeSet) *Builder {
	return &Builder{
		repoPath:    repoPath,
		artifactDir: artifactDir,
		config:      cfg,
		changeset:   cs,
		result:      &BuildResult{},
	}
}

// Build executes all necessary builds based on the changeset
func (b *Builder) Build() (*BuildResult, error) {
	// Step 1: Copy entire repository to app/ directory (including ignored paths for build)
	fmt.Println("→ Copying project files to artifact...")
	if err := b.copyEntireRepo(); err != nil {
		return nil, fmt.Errorf("failed to copy repository: %w", err)
	}

	// Step 2: Build PHP (runs composer, updates vendor in place)
	if b.config.Builds.PHP.Enabled {
		if err := b.buildPHP(); err != nil {
			return nil, fmt.Errorf("php build failed: %w", err)
		}
	}

	// Step 3: Build Go (creates binary)
	if b.config.Builds.Go.Enabled {
		if err := b.buildGo(); err != nil {
			return nil, fmt.Errorf("go build failed: %w", err)
		}
	}

	// Step 4: Build Frontend (runs npm, compiles, updates node_modules)
	if b.config.Builds.Frontend.Enabled {
		if err := b.buildFrontend(); err != nil {
			return nil, fmt.Errorf("frontend build failed: %w", err)
		}
	}

	// Step 5: Cleanup ignored paths after builds complete
	fmt.Println("→ Cleaning up build-time dependencies...")
	if err := b.cleanupIgnoredPaths(); err != nil {
		return nil, fmt.Errorf("failed to cleanup ignored paths: %w", err)
	}

	return b.result, nil
}

// copyEntireRepo copies the entire repository to app/ directory (including ignored paths for build)
func (b *Builder) copyEntireRepo() error {
	appDir := filepath.Join(b.artifactDir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	// Walk through the repository and copy EVERYTHING (we'll cleanup ignored paths after build)
	return filepath.Walk(b.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(b.repoPath, path)
		if err != nil {
			return err
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Skip .git directory (always ignore)
		if strings.HasPrefix(relPath, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Destination path in artifact
		dstPath := filepath.Join(appDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// cleanupIgnoredPaths removes ignored paths from artifact after builds complete
func (b *Builder) cleanupIgnoredPaths() error {
	appDir := filepath.Join(b.artifactDir, "app")

	for _, ignored := range b.config.Ignored {
		// Skip .git as it's already not copied
		if ignored == ".git" {
			continue
		}

		ignoredPath := filepath.Join(appDir, ignored)

		// Check if path exists
		if _, err := os.Stat(ignoredPath); os.IsNotExist(err) {
			continue // Path doesn't exist, skip
		}

		// Remove the path
		if err := os.RemoveAll(ignoredPath); err != nil {
			return fmt.Errorf("failed to remove ignored path %s: %w", ignored, err)
		}
		fmt.Printf("   Removed: %s\n", ignored)
	}

	return nil
}

// buildPHP handles PHP builds
func (b *Builder) buildPHP() error {
	// Run composer if composer.json changed
	if b.changeset.ComposerChanged {
		fmt.Println("→ Running composer install...")

		// Run composer in the artifact's app directory
		composerDir := filepath.Join(b.artifactDir, "app", b.config.Builds.PHP.ProjectRoot)
		fmt.Printf("   Working directory: app/%s\n", b.config.Builds.PHP.ProjectRoot)

		output, err := b.executeCommand(b.config.Builds.PHP.ComposerCommand, composerDir)
		if err != nil {
			fmt.Printf("   Composer output:\n%s\n", string(output))
			return verserrors.New(verserrors.CodeBuildFailed, "Composer command failed", "Check your composer.json and ensure all dependencies are available locally.", fmt.Errorf("%w: %s", err, string(output)))
		}
		fmt.Println("   ✓ Composer install completed")

		b.result.ComposerUpdated = true
	}

	// Count PHP files (already copied by copyEntireRepo)
	b.result.PHPFilesChanged = len(b.changeset.PHPFiles)
	b.result.TwigCacheCleanup = len(b.changeset.TwigFiles) > 0
	b.result.RouteCacheRegenerate = b.changeset.RoutesChanged

	return nil
}

// buildGo handles Go builds
func (b *Builder) buildGo() error {
	if !b.changeset.GoModChanged && len(b.changeset.GoFiles) == 0 {
		return nil // No Go changes
	}

	fmt.Println("→ Building Go binary...")

	goCfg := b.config.Builds.Go
	binaryPath := filepath.Join(b.artifactDir, "bin", goCfg.BinaryName)

	// Prepare build command
	buildCmd := fmt.Sprintf("GOOS=%s GOARCH=%s go build -o %s", goCfg.TargetOS, goCfg.TargetArch, binaryPath)
	if goCfg.BuildFlags != "" {
		buildCmd = fmt.Sprintf("GOOS=%s GOARCH=%s go build %s -o %s", goCfg.TargetOS, goCfg.TargetArch, goCfg.BuildFlags, binaryPath)
	}

	output, err := b.executeCommand(buildCmd, filepath.Join(b.repoPath, b.config.Builds.Go.ProjectRoot))
	if err != nil {
		return verserrors.New(verserrors.CodeBuildFailed, "Go build failed", "Check your Go code for compilation errors and ensure all dependencies are resolved.", fmt.Errorf("%w: %s", err, string(output)))
	}

	// Validate binary was created
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("go binary not created: %s", binaryPath)
	}

	b.result.GoBinaryRebuilt = true
	return nil
}

// buildFrontend handles frontend builds
func (b *Builder) buildFrontend() error {
	// Run npm if package.json changed or if we need to compile but node_modules is missing
	npmDir := filepath.Join(b.artifactDir, "app", b.config.Builds.Frontend.ProjectRoot)
	nmPath := filepath.Join(npmDir, "node_modules")

	needsInstall := b.changeset.PackageChanged
	if !needsInstall && len(b.changeset.FrontendFiles) > 0 {
		if _, err := os.Stat(nmPath); os.IsNotExist(err) {
			needsInstall = true
		}
	}

	if needsInstall {
		fmt.Println("→ Running npm install...")
		fmt.Printf("   Working directory: app/%s\n", b.config.Builds.Frontend.ProjectRoot)

		output, err := b.executeCommand(b.config.Builds.Frontend.NPMCommand, npmDir)
		if err != nil {
			fmt.Printf("   NPM output:\n%s\n", string(output))
			return verserrors.New(verserrors.CodeBuildFailed, "NPM command failed", "Check your package.json and ensure npm/node is installed correctly.", fmt.Errorf("%w: %s", err, string(output)))
		}
		fmt.Println("   ✓ NPM install completed")

		b.result.NPMUpdated = true
	}

	// If compile_command doesn't contain {file}, run it once if any frontend files changed
	if !strings.Contains(b.config.Builds.Frontend.CompileCommand, "{file}") {
		if len(b.changeset.FrontendFiles) > 0 {
			fmt.Println("→ Compiling frontend (global)...")

			// Run compile in the artifact's app directory
			compileDir := filepath.Join(b.artifactDir, "app", b.config.Builds.Frontend.ProjectRoot)
			fmt.Printf("   Working directory: app/%s\n", b.config.Builds.Frontend.ProjectRoot)
			fmt.Printf("   Command: %s\n", b.config.Builds.Frontend.CompileCommand)

			output, err := b.executeCommand(b.config.Builds.Frontend.CompileCommand, compileDir)
			if err != nil {
				fmt.Printf("   Compilation output:\n%s\n", string(output))
				return verserrors.New(verserrors.CodeBuildFailed, "Frontend compile failed", "Check your build command.", fmt.Errorf("%w: %s", err, string(output)))
			}
			fmt.Printf("   Compilation output:\n%s\n", string(output))
			fmt.Println("   ✓ Frontend compilation completed")
			b.result.FrontendCompiled = len(b.changeset.FrontendFiles)
		}

		// Cleanup dev dependencies if enabled
		if err := b.cleanupDevDependencies(); err != nil {
			return err
		}

		return nil
	}

	// Compile changed frontend files individually
	for _, file := range b.changeset.FrontendFiles {
		fmt.Printf("→ Compiling %s...\n", file)

		// Replace {file} placeholder in compile command
		compileCmd := strings.Replace(b.config.Builds.Frontend.CompileCommand, "{file}", file, -1)
		compileDir := filepath.Join(b.artifactDir, "app", b.config.Builds.Frontend.ProjectRoot)
		fmt.Printf("   Command: %s\n", compileCmd)

		output, err := b.executeCommand(compileCmd, compileDir)
		if err != nil {
			fmt.Printf("   Compilation output:\n%s\n", string(output))
			return verserrors.New(verserrors.CodeBuildFailed, fmt.Sprintf("Compile failed for %s", file), "Check your custom compiler command and ensure it's correct for this file type.", fmt.Errorf("%w: %s", err, string(output)))
		}
		fmt.Printf("   ✓ Compiled successfully\n")

		b.result.FrontendCompiled++
	}

	// Cleanup dev dependencies if enabled
	if err := b.cleanupDevDependencies(); err != nil {
		return err
	}

	return nil
}

// cleanupDevDependencies removes dev dependencies and reinstalls production-only packages
func (b *Builder) cleanupDevDependencies() error {
	if !b.config.Builds.Frontend.CleanupDevDeps {
		return nil // Feature not enabled
	}

	if !b.changeset.PackageChanged {
		return nil // No package changes, skip cleanup
	}

	fmt.Println("→ Cleaning up dev dependencies...")

	// Remove node_modules from artifact
	nodeModulesDst := filepath.Join(b.artifactDir, "app", b.config.Builds.Frontend.ProjectRoot, "node_modules")
	if err := os.RemoveAll(nodeModulesDst); err != nil {
		return fmt.Errorf("failed to remove node_modules from artifact: %w", err)
	}

	// Run production install in the artifact
	fmt.Println("→ Installing production dependencies only...")
	productionDir := filepath.Join(b.artifactDir, "app", b.config.Builds.Frontend.ProjectRoot)
	fmt.Printf("   Working directory: app/%s\n", b.config.Builds.Frontend.ProjectRoot)
	fmt.Printf("   Command: %s\n", b.config.Builds.Frontend.ProductionCommand)

	output, err := b.executeCommand(b.config.Builds.Frontend.ProductionCommand, productionDir)
	if err != nil {
		fmt.Printf("   Production install output:\n%s\n", string(output))
		return verserrors.New(verserrors.CodeBuildFailed, "Production install failed", "Check your production_command configuration.", fmt.Errorf("%w: %s", err, string(output)))
	}
	fmt.Println("   ✓ Production dependencies installed")

	fmt.Println("→ Dev dependencies cleaned up successfully")
	return nil
}

// executeCommand runs a command in a shell based on the current OS
func (b *Builder) executeCommand(command, dir string) ([]byte, error) {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		flag = "/c"
	} else {
		shell = "sh"
		flag = "-c"
	}

	cmd := exec.Command(shell, flag, command)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// copyFile copies a single file using io.Copy for efficiency and reliability
func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Double check it's a regular file. We should NEVER try to read directories as files.
	// This prevents "Función incorrecta" errors on Windows for junctions/reparse points.
	if !info.Mode().IsRegular() {
		// If it's a symlink that made it here, evaluate it
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(src)
			if err != nil {
				return nil // Skip if broken
			}
			return copyFile(realPath, dst)
		}
		return nil // Skip non-regular files
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy permissions
	os.Chmod(dst, info.Mode())

	return nil
}

// copyDir recursively copies a directory, flattening symlinks for the artifact
func copyDir(src, dst string) error {
	// Root directory creation
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Handle Symlinks/Junctions by following them (flattening)
		if info.Mode()&os.ModeSymlink != 0 || (runtime.GOOS == "windows" && (info.Mode()&os.ModeDevice != 0)) {
			realPath, err := filepath.EvalSymlinks(srcPath)
			if err != nil {
				continue
			}

			realInfo, err := os.Stat(realPath)
			if err != nil {
				continue
			}

			if realInfo.IsDir() {
				if err := copyDir(realPath, dstPath); err != nil {
					return err
				}
			} else {
				if err := copyFile(realPath, dstPath); err != nil {
					return err
				}
			}
			continue
		}

		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
