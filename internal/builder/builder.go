package builder

import (
	"fmt"
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
	// Create artifact directory structure
	if err := b.createArtifactStructure(); err != nil {
		return nil, err
	}

	// Build PHP
	if b.config.Builds.PHP.Enabled {
		if err := b.buildPHP(); err != nil {
			return nil, fmt.Errorf("php build failed: %w", err)
		}
	}

	// Build Go
	if b.config.Builds.Go.Enabled {
		if err := b.buildGo(); err != nil {
			return nil, fmt.Errorf("go build failed: %w", err)
		}
	}

	// Build Frontend
	if b.config.Builds.Frontend.Enabled {
		if err := b.buildFrontend(); err != nil {
			return nil, fmt.Errorf("frontend build failed: %w", err)
		}
	}

	// Copy other changed files
	if err := b.copyOtherFiles(); err != nil {
		return nil, fmt.Errorf("failed to copy other files: %w", err)
	}

	return b.result, nil
}

// createArtifactStructure creates the artifact directory layout
func (b *Builder) createArtifactStructure() error {
	dirs := []string{
		filepath.Join(b.artifactDir, "app"),
		filepath.Join(b.artifactDir, "vendor"),
		filepath.Join(b.artifactDir, "node_modules"),
		filepath.Join(b.artifactDir, "public"),
		filepath.Join(b.artifactDir, "bin"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// buildPHP handles PHP builds
func (b *Builder) buildPHP() error {
	// Run composer if composer.json changed
	if b.changeset.ComposerChanged {
		fmt.Println("→ Running composer install...")
		output, err := b.executeCommand(b.config.Builds.PHP.ComposerCommand, filepath.Join(b.repoPath, b.config.Builds.PHP.ProjectRoot))
		if err != nil {
			return verserrors.New(verserrors.CodeBuildFailed, "Composer command failed", "Check your composer.json and ensure all dependencies are available locally.", fmt.Errorf("%w: %s", err, string(output)))
		}

		// Copy vendor directory
		vendorSrc := filepath.Join(b.repoPath, b.config.Builds.PHP.ProjectRoot, "vendor")
		vendorDst := filepath.Join(b.artifactDir, "vendor")
		if _, err := os.Stat(vendorSrc); err == nil {
			if err := copyDir(vendorSrc, vendorDst); err != nil {
				return fmt.Errorf("failed to copy vendor directory: %w", err)
			}
		}

		b.result.ComposerUpdated = true
	}

	// Copy changed PHP files
	for _, file := range b.changeset.PHPFiles {
		src := filepath.Join(b.repoPath, file)
		dst := filepath.Join(b.artifactDir, "app", file)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", file, err)
		}

		b.result.PHPFilesChanged++
	}

	// Copy changed Twig files
	for _, file := range b.changeset.TwigFiles {
		src := filepath.Join(b.repoPath, file)
		dst := filepath.Join(b.artifactDir, "app", file)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", file, err)
		}

		b.result.TwigCacheCleanup = true
	}

	// Mark route cache regeneration if needed
	if b.changeset.RoutesChanged {
		b.result.RouteCacheRegenerate = true
	}

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
	// Run npm if package.json changed
	if b.changeset.PackageChanged {
		fmt.Println("→ Running npm ci...")
		output, err := b.executeCommand(b.config.Builds.Frontend.NPMCommand, filepath.Join(b.repoPath, b.config.Builds.Frontend.ProjectRoot))
		if err != nil {
			return verserrors.New(verserrors.CodeBuildFailed, "NPM command failed", "Check your package.json and ensure npm/node is installed correctly.", fmt.Errorf("%w: %s", err, string(output)))
		}

		// Copy node_modules directory
		nodeModulesSrc := filepath.Join(b.repoPath, b.config.Builds.Frontend.ProjectRoot, "node_modules")
		nodeModulesDst := filepath.Join(b.artifactDir, "node_modules")
		if _, err := os.Stat(nodeModulesSrc); err == nil {
			if err := copyDir(nodeModulesSrc, nodeModulesDst); err != nil {
				return fmt.Errorf("failed to copy node_modules: %w", err)
			}
		}

		b.result.NPMUpdated = true
	}

	// If compile_command doesn't contain {file}, run it once if any frontend files changed
	if !strings.Contains(b.config.Builds.Frontend.CompileCommand, "{file}") {
		if len(b.changeset.FrontendFiles) > 0 {
			fmt.Println("→ Compiling frontend (global)...")
			output, err := b.executeCommand(b.config.Builds.Frontend.CompileCommand, filepath.Join(b.repoPath, b.config.Builds.Frontend.ProjectRoot))
			if err != nil {
				return verserrors.New(verserrors.CodeBuildFailed, "Frontend compile failed", "Check your build command.", fmt.Errorf("%w: %s", err, string(output)))
			}
			b.result.FrontendCompiled = len(b.changeset.FrontendFiles)
		}
		return nil
	}

	// Compile changed frontend files individually
	for _, file := range b.changeset.FrontendFiles {
		fmt.Printf("→ Compiling %s...\n", file)

		// Replace {file} placeholder in compile command
		compileCmd := strings.Replace(b.config.Builds.Frontend.CompileCommand, "{file}", file, -1)

		output, err := b.executeCommand(compileCmd, filepath.Join(b.repoPath, b.config.Builds.Frontend.ProjectRoot))
		if err != nil {
			return verserrors.New(verserrors.CodeBuildFailed, fmt.Sprintf("Compile failed for %s", file), "Check your custom compiler command and ensure it's correct for this file type.", fmt.Errorf("%w: %s", err, string(output)))
		}

		// Copy compiled output to artifact (assuming it's in public/)
		// This is a simplification - actual output path may vary
		src := filepath.Join(b.repoPath, "public", filepath.Base(file))
		dst := filepath.Join(b.artifactDir, "public", filepath.Base(file))

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(src, dst); err != nil {
			// If file doesn't exist in public/, try the original location
			src = filepath.Join(b.repoPath, file)
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("failed to copy compiled %s: %w", file, err)
			}
		}

		b.result.FrontendCompiled++
	}

	return nil
}

// copyOtherFiles copies files that don't fall into specific categories
func (b *Builder) copyOtherFiles() error {
	for _, file := range b.changeset.OtherFiles {
		src := filepath.Join(b.repoPath, file)
		dst := filepath.Join(b.artifactDir, "app", file)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy other file %s: %w", file, err)
		}
	}
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

// copyFile copies a single file
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, input, 0644); err != nil {
		return err
	}

	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}
