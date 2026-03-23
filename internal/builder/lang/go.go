package lang

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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

	// Prepare build command
	buildCmd := fmt.Sprintf("GOOS=%s GOARCH=%s go build -o %s", goCfg.TargetOS, goCfg.TargetArch, binaryPath)
	if goCfg.BuildFlags != "" {
		buildCmd = fmt.Sprintf("GOOS=%s GOARCH=%s go build %s -o %s", goCfg.TargetOS, goCfg.TargetArch, goCfg.BuildFlags, binaryPath)
	}

	output, err := executeCommand(buildCmd, filepath.Join(ctx.RepoPath, goCfg.ProjectRoot))
	if err != nil {
		return 0, false, verserrors.New(verserrors.CodeBuildFailed, "Go build failed", "Check your Go code for compilation errors and ensure all dependencies are resolved.", fmt.Errorf("%w: %s", err, string(output)))
	}

	// Validate binary was created
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return 0, false, fmt.Errorf("go binary not created: %s", binaryPath)
	}

	return 0, true, nil
}

// executeCommand runs a command in a shell based on the current OS
func executeCommand(command, dir string) ([]byte, error) {
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
