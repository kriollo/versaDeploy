package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
)

// PythonBuilder implements LanguageBuilder for Python environments
type PythonBuilder struct{}

// Build writes run_server.sh (web_server) and builds the PyInstaller binary
// (build_binary). Dependencies are not installed here: the deployer creates the
// virtualenv on the server, since one built on this machine wouldn't run there.
func (p *PythonBuilder) Build(ctx *BuilderContext) (int, bool, error) {
	cfg := ctx.Config.Builds.Python
	appDir := filepath.Join(ctx.ArtifactDir, "app", cfg.ProjectRoot)

	// Always written (it's tiny) so changes to web_* settings apply without code changes
	if cfg.WebServer {
		if err := os.MkdirAll(appDir, 0775); err != nil {
			return 0, false, fmt.Errorf("failed to create Python project directory: %w", err)
		}
		if err := p.setupWebServer(ctx, appDir, cfg); err != nil {
			return 0, false, err
		}
	}

	if !cfg.BuildBinary || (len(ctx.Changeset.PythonFiles) == 0 && !ctx.Changeset.RequirementsChanged && !ctx.Changeset.Force) {
		return 0, cfg.WebServer, nil
	}
	if runtime.GOOS != "linux" {
		return 0, false, verserrors.New(verserrors.CodeBuildFailed, "python.build_binary needs versa to run on Linux",
			"PyInstaller can't cross-compile: on "+runtime.GOOS+" it builds a binary for "+runtime.GOOS+", which the server can't run. Deploy from Linux (or WSL/CI), or disable build_binary to run the code with a virtualenv on the server.", nil)
	}
	// PyInstaller bundles the packages installed where it runs
	if err := p.installDependencies(ctx, appDir, cfg); err != nil {
		return 0, false, err
	}
	if err := p.buildBinary(ctx, appDir, cfg); err != nil {
		return 0, false, err
	}
	ctx.Log.Info("Python binary built: %s", cfg.BinaryName)
	return 1, true, nil
}

func (p *PythonBuilder) installDependencies(ctx *BuilderContext, appDir string, cfg config.PythonBuildConfig) error {
	var installCmd string
	var args []string
	var extraIndexUrls []string

	hasTorch := false
	reqFile := filepath.Join(appDir, cfg.RequirementsFile)
	if data, err := os.ReadFile(reqFile); err == nil {
		hasTorch = strings.Contains(string(data), "torch")
	}

	switch cfg.PackageManager {
	case "poetry":
		installCmd = "poetry"
		args = []string{"install"}
		if cfg.InstallDevDeps {
			args = append(args, "--with", "dev")
		} else {
			args = append(args, "--no-dev")
		}
	case "pipenv":
		installCmd = "pipenv"
		args = []string{"install", "--deploy"}
		if !cfg.InstallDevDeps {
			args = append(args, "--prod")
		}
	default:
		installCmd = cfg.PythonCommand
		if installCmd == "" {
			installCmd = "python3"
		}

		if _, err := os.Stat(reqFile); err == nil {
			args = []string{"-m", "pip", "install", "-r", cfg.RequirementsFile}
			if cfg.UseCache {
				args = append(args, "--cache-dir", "/tmp/pip-cache")
			}

			// Handle PyTorch index URL
			if cfg.TorchIndex != "" && hasTorch {
				extraIndexUrls = append(extraIndexUrls, cfg.TorchIndex)
				args = append(args, "--extra-index-url", cfg.TorchIndex)
			} else if cfg.TorchIndex != "" {
				args = append(args, "-i", cfg.TorchIndex)
			} else if cfg.PyPIMirror != "" {
				args = append(args, "-i", cfg.PyPIMirror)
			}
		} else {
			ctx.Log.Debug("No requirements.txt found, skipping pip install")
			return nil
		}
	}

	if installCmd != "" {
		ctx.Log.Info("Installing Python dependencies with %s...", cfg.PackageManager)

		output, err := executeCommand(installCmd+" "+strings.Join(args, " "), appDir)
		if err != nil {
			ctx.Log.Debug("Python install output: %s", string(output))
			return fmt.Errorf("failed to install Python dependencies: %w", err)
		}
	}

	// Install extra requirements files (after main deps)
	for _, extraReq := range cfg.ExtraRequirements {
		extraReqFile := filepath.Join(appDir, extraReq)
		if _, err := os.Stat(extraReqFile); err == nil {
			ctx.Log.Info("Installing extra requirements: %s", extraReq)
			extraArgs := []string{"-m", "pip", "install", "-r", extraReq}
			if cfg.TorchIndex != "" && hasTorch {
				extraArgs = append(extraArgs, "--extra-index-url", cfg.TorchIndex)
			}

			output, err := executeCommand(installCmd+" "+strings.Join(extraArgs, " "), appDir)
			if err != nil {
				ctx.Log.Debug("Extra requirements install output: %s", string(output))
				return fmt.Errorf("failed to install extra requirements %s: %w", extraReq, err)
			}
		}
	}

	return nil
}

func (p *PythonBuilder) buildBinary(ctx *BuilderContext, appDir string, cfg config.PythonBuildConfig) error {
	ctx.Log.Info("Building Python binary with PyInstaller...")

	pyCmd := cfg.PythonCommand
	if pyCmd == "" {
		pyCmd = "python3"
	}

	args := []string{
		"-m", "PyInstaller",
		"--name", cfg.BinaryName,
		"--onefile",
		"--distpath", appDir,
		"--workpath", filepath.Join(appDir, "build"),
		"--specpath", appDir,
	}

	if cfg.ExtraPyinstallerArgs != "" {
		args = append(args, strings.Fields(cfg.ExtraPyinstallerArgs)...)
	}

	args = append(args, cfg.EntryPoint)

	output, err := executeCommand(pyCmd+" "+strings.Join(args, " "), appDir)
	if err != nil {
		ctx.Log.Debug("PyInstaller output: %s", string(output))
		return fmt.Errorf("failed to build Python binary: %w", err)
	}

	os.RemoveAll(filepath.Join(appDir, "build"))
	os.Remove(filepath.Join(appDir, cfg.BinaryName+".spec"))
	return nil
}

func (p *PythonBuilder) setupWebServer(ctx *BuilderContext, appDir string, cfg config.PythonBuildConfig) error {
	wsgi := ""
	if cfg.WebFramework == "django" && cfg.RunCommand == "" {
		if wsgi = findDjangoWSGI(appDir); wsgi == "" {
			ctx.Log.Warn("Django: no <project>/wsgi.py found, falling back to manage.py runserver (development server). Set python.run_command for production.")
		}
	}
	scriptPath := filepath.Join(appDir, "run_server.sh")
	if err := os.WriteFile(scriptPath, []byte(runServerScript(cfg, wsgi)), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}
	ctx.Log.Debug("Generated run_server.sh")
	return nil
}

// findDjangoWSGI returns the WSGI module of a Django project ("mysite.wsgi"), found as
// <dir>/wsgi.py next to manage.py; "" if there isn't exactly one.
func findDjangoWSGI(appDir string) string {
	matches, _ := filepath.Glob(filepath.Join(appDir, "*", "wsgi.py"))
	if len(matches) != 1 {
		return ""
	}
	return filepath.Base(filepath.Dir(matches[0])) + ".wsgi"
}

// runServerScript returns run_server.sh: it runs from its own directory (the Python
// root) with the release's virtualenv, and execs the server so the init system tracks
// its PID. Migrations/collectstatic belong in post_deploy hooks, not here.
func runServerScript(cfg config.PythonBuildConfig, djangoWSGI string) string {
	py := filepath.ToSlash(cfg.VenvPath) + "/bin/python"
	entry := strings.TrimSuffix(filepath.ToSlash(cfg.EntryPoint), ".py")
	entry = strings.ReplaceAll(entry, "/", ".")
	bind := fmt.Sprintf("%s:%d", cfg.WebHost, cfg.WebPort)
	workers := cfg.WebWorkers

	var run string
	switch {
	case cfg.RunCommand != "":
		run = cfg.RunCommand
	case cfg.WebFramework == "django" && djangoWSGI != "":
		if workers == 0 {
			workers = 2
		}
		run = fmt.Sprintf("%s -m gunicorn %s:application -w %d -b %s", py, djangoWSGI, workers, bind)
	case cfg.WebFramework == "django":
		run = fmt.Sprintf("%s manage.py runserver %s --noreload", py, bind)
	case cfg.WebFramework == "flask":
		run = fmt.Sprintf("FLASK_APP=%s %s -m flask run --host=%s --port=%d", cfg.EntryPoint, py, cfg.WebHost, cfg.WebPort)
	case cfg.WebFramework == "fastapi" || cfg.WebFramework == "uvicorn":
		run = fmt.Sprintf("%s -m uvicorn %s:app --host %s --port %d", py, entry, cfg.WebHost, cfg.WebPort)
		if workers > 0 {
			run += fmt.Sprintf(" --workers %d", workers)
		}
	case cfg.WebFramework == "gunicorn":
		if workers == 0 {
			workers = 4
		}
		run = fmt.Sprintf("%s -m gunicorn %s:app -w %d -b %s", py, entry, workers, bind)
		if cfg.WebThreads > 0 {
			run += fmt.Sprintf(" --threads %d", cfg.WebThreads)
		}
	case cfg.EntryPoint != "":
		run = fmt.Sprintf("%s %s", py, cfg.EntryPoint)
	default:
		run = fmt.Sprintf("%s -m http.server %d", py, cfg.WebPort)
	}
	return "#!/bin/sh\n# Generated by versa. Runs with the release's virtualenv.\ncd \"$(dirname \"$0\")\" || exit 1\nexport PYTHONUNBUFFERED=1\nexec " + run + "\n"
}
