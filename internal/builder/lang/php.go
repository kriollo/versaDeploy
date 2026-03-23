package lang

import (
	"fmt"
	"path/filepath"

	verserrors "github.com/user/versaDeploy/internal/errors"
)

// PHPBuilder implements LanguageBuilder for PHP projects
type PHPBuilder struct{}

// Build runs Composer on the PHP backend if required
func (p *PHPBuilder) Build(ctx *BuilderContext) (int, bool, error) {
	isUpdated := false
	if ctx.Changeset.ComposerChanged || ctx.Changeset.Force {
		ctx.Log.Info("Running composer install...")

		composerDir := filepath.Join(ctx.ArtifactDir, "app", ctx.Config.Builds.PHP.ProjectRoot)
		ctx.Log.Debug("   Working directory: app/%s", ctx.Config.Builds.PHP.ProjectRoot)

		output, err := executeCommand(ctx.Config.Builds.PHP.ComposerCommand, composerDir)
		if err != nil {
			ctx.Log.Debug("Composer output:\n%s", string(output))
			return 0, false, verserrors.New(verserrors.CodeBuildFailed, "Composer command failed", "Check your composer.json and ensure all dependencies are available locally.", fmt.Errorf("%w: %s", err, string(output)))
		}
		ctx.Log.Success("Composer install completed")
		isUpdated = true
	}

	return len(ctx.Changeset.PHPFiles), isUpdated, nil
}
