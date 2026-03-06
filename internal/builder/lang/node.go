package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/fsutil"
)

// FrontendBuilder implements LanguageBuilder for Javascript/Node projects
type FrontendBuilder struct{}

// Build compiles JS/CSS assets and cleans up dev dependencies
func (f *FrontendBuilder) Build(ctx *BuilderContext) (int, bool, error) {
	npmDir := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.Frontend.ProjectRoot)
	nmPath := filepath.Join(npmDir, "node_modules")
	isUpdated := false
	filesCompiled := 0

	needsInstall := ctx.Changeset.PackageChanged || ctx.Changeset.Force
	if !needsInstall && len(ctx.Changeset.FrontendFiles) > 0 {
		if _, err := os.Stat(nmPath); os.IsNotExist(err) {
			needsInstall = true
		}
	}

	if needsInstall {
		ctx.Log.Info("Running npm install...")
		ctx.Log.Debug("   Working directory: app/%s", ctx.Config.Builds.Frontend.ProjectRoot)

		output, err := executeCommand(ctx.Config.Builds.Frontend.NPMCommand, npmDir)
		if err != nil {
			ctx.Log.Debug("NPM output:\n%s", string(output))
			return 0, false, verserrors.New(verserrors.CodeBuildFailed, "NPM command failed", "Check your package.json and ensure npm/node is installed correctly.", fmt.Errorf("%w: %s", err, string(output)))
		}
		ctx.Log.Success("NPM install completed")
		isUpdated = true
	}

	// If compile_command doesn't contain {file}, run it once if any frontend files changed
	if !strings.Contains(ctx.Config.Builds.Frontend.CompileCommand, "{file}") {
		if len(ctx.Changeset.FrontendFiles) > 0 || ctx.Changeset.Force {
			ctx.Log.Info("Compiling frontend assets...")
			compileDir := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.Frontend.ProjectRoot)
			ctx.Log.Debug("   Command: %s", ctx.Config.Builds.Frontend.CompileCommand)

			output, err := executeCommand(ctx.Config.Builds.Frontend.CompileCommand, compileDir)
			if err != nil {
				ctx.Log.Debug("Compilation output:\n%s", string(output))
				return 0, isUpdated, verserrors.New(verserrors.CodeBuildFailed, "Frontend compile failed", "Check your build command.", fmt.Errorf("%w: %s", err, string(output)))
			}
			ctx.Log.Success("Frontend compilation completed")
			filesCompiled = len(ctx.Changeset.FrontendFiles)
		}
	} else {
		// Compile changed frontend files individually
		for _, file := range ctx.Changeset.FrontendFiles {
			ctx.Log.Info("Compiling frontend asset: %s", file)
			compileCmd := strings.Replace(ctx.Config.Builds.Frontend.CompileCommand, "{file}", file, -1)
			compileDir := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.Frontend.ProjectRoot)

			output, err := executeCommand(compileCmd, compileDir)
			if err != nil {
				ctx.Log.Debug("Compilation output:\n%s", string(output))
				return filesCompiled, isUpdated, verserrors.New(verserrors.CodeBuildFailed, fmt.Sprintf("Compile failed for %s", file), "Check your custom compiler command and ensure it's correct for this file type.", fmt.Errorf("%w: %s", err, string(output)))
			}
			ctx.Log.Success("Compiled successfully: %s", file)
			filesCompiled++
		}
	}

	// Cleanup dev dependencies if enabled
	if err := f.cleanupDevDependencies(ctx); err != nil {
		return filesCompiled, isUpdated, err
	}

	return filesCompiled, isUpdated, nil
}

func (f *FrontendBuilder) cleanupDevDependencies(ctx *BuilderContext) error {
	if !ctx.Config.Builds.Frontend.CleanupDevDeps {
		return nil
	}

	if !ctx.Changeset.PackageChanged && len(ctx.Changeset.FrontendFiles) == 0 && !ctx.Changeset.Force {
		return nil
	}

	ctx.Log.Info("Cleaning up dev dependencies...")
	nodeModulesDst := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.Frontend.ProjectRoot, "node_modules")

	beforeSize, _ := fsutil.CalculateDirSize(nodeModulesDst)
	ctx.Log.Debug("   Size before cleanup: %d MB", beforeSize/(1024*1024))

	if err := os.RemoveAll(nodeModulesDst); err != nil {
		return fmt.Errorf("failed to remove node_modules from artifact: %w", err)
	}

	ctx.Log.Info("Installing production dependencies...")
	productionDir := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.Frontend.ProjectRoot)

	output, err := executeCommand(ctx.Config.Builds.Frontend.ProductionCommand, productionDir)
	if err != nil {
		ctx.Log.Debug("Production install output:\n%s", string(output))
		return verserrors.New(verserrors.CodeBuildFailed, "Production install failed", "Check your production_command configuration.", fmt.Errorf("%w: %s", err, string(output)))
	}

	if len(output) > 0 {
		ctx.Log.Debug("   Command output: %s", strings.TrimSpace(string(output)))
	}

	afterSize, _ := fsutil.CalculateDirSize(nodeModulesDst)
	ctx.Log.Debug("   Size after cleanup: %d MB", afterSize/(1024*1024))
	ctx.Log.Success("Dev dependencies cleaned up successfully")

	return nil
}
