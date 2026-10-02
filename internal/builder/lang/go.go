package lang

import (
	"fmt"
	"os"
	"path/filepath"

	verserrors "github.com/user/versaDeploy/internal/errors"
)

// GoBuilder implements LanguageBuilder for Go projects
type GoBuilder struct{}

// Build compiles the Go binary
func (g *GoBuilder) Build(ctx *BuilderContext) (int, bool, error) {
	if !ctx.Changeset.GoModChanged && len(ctx.Changeset.GoFiles) == 0 {
		return 0, false, nil // No Go changes
	}

	goCfg := ctx.Config.Builds.Go
	binaryPath := filepath.Join(ctx.ArtifactDir, goCfg.DeployPath, goCfg.BinaryName)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0775); err != nil {
		return 0, false, fmt.Errorf("failed to create Go output directory: %w", err)
	}

	ctx.Log.Info("Building Go binary: %s", goCfg.BinaryName)

	// Target settings go through the process env: inline `VAR=x cmd` doesn't work in cmd.exe.
	// CGO is off by default so the binary is static and doesn't depend on the build
	// machine's glibc (a newer glibc than the server's fails with "GLIBC_2.xx not found").
	cgo := "0"
	if goCfg.CGO {
		cgo = "1"
	}
	env := []string{"GOOS=" + goCfg.TargetOS, "GOARCH=" + goCfg.TargetArch, "CGO_ENABLED=" + cgo}

	buildCmd := "go build"
	if goCfg.BuildFlags != "" {
		buildCmd += " " + goCfg.BuildFlags
	}
	buildCmd += ` -o "` + binaryPath + `"`

	output, err := executeCommand(buildCmd, filepath.Join(ctx.RepoPath, goCfg.ProjectRoot), env...)
	if err != nil {
		return 0, false, verserrors.New(verserrors.CodeBuildFailed, "Go build failed", "Check your Go code for compilation errors and ensure all dependencies are resolved.", fmt.Errorf("%w: %s", err, string(output)))
	}

	// Validate binary was created
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return 0, false, fmt.Errorf("go binary not created: %s", binaryPath)
	}

	return 0, true, nil
}

// executeCommand runs a command in the OS shell (cmd.exe on Windows, sh elsewhere)
func executeCommand(command, dir string, env ...string) ([]byte, error) {
	cmd := shellCommand(command)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.CombinedOutput()
}
