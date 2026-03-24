package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/versaDeploy/internal/config"
)

// PythonBuilder implements LanguageBuilder for Python environments
type PythonBuilder struct{}

// Build sets up pip/poetry dependencies, PyInstaller, and Gunicorn/Uvicorn systemd services
func (p *PythonBuilder) Build(ctx *BuilderContext) (int, bool, error) {
	if len(ctx.Changeset.PythonFiles) == 0 && !ctx.Changeset.RequirementsChanged {
		ctx.Log.Debug("No Python files changed, skipping build")
		return 0, false, nil
	}

	cfg := ctx.Config.Builds.Python
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		projectRoot = "."
	}

	appDir := filepath.Join(ctx.ArtifactDir, "app", projectRoot)

	if err := os.MkdirAll(appDir, 0775); err != nil {
		return 0, false, fmt.Errorf("failed to create Python project directory: %w", err)
	}

	if err := p.installDependencies(ctx, appDir, cfg); err != nil {
		return 0, false, err
	}

	filesBuilt := 0
	if cfg.BuildBinary {
		if err := p.buildBinary(ctx, appDir, cfg); err != nil {
			return 0, false, err
		}
		filesBuilt = 1
	}

	if cfg.WebServer {
		if err := p.setupWebServer(ctx, appDir, cfg); err != nil {
			return 0, false, err
		}
	}

	ctx.Log.Info("Python build completed: %d files", len(ctx.Changeset.PythonFiles))
	return filesBuilt, true, nil
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

	ctx.Log.Info("Python binary built: %s", cfg.BinaryName)
	return nil
}

func (p *PythonBuilder) setupWebServer(ctx *BuilderContext, appDir string, cfg config.PythonBuildConfig) error {
	ctx.Log.Info("Setting up Python web server...")

	var runCmd string
	if cfg.RunCommand != "" {
		runCmd = cfg.RunCommand
	} else {
		switch cfg.WebFramework {
		case "django":
			runCmd = fmt.Sprintf("%s manage.py migrate --no-input && %s manage.py collectstatic --no-input && %s manage.py runserver %s:%d",
				cfg.PythonCommand, cfg.PythonCommand, cfg.PythonCommand, cfg.WebHost, cfg.WebPort)
		case "flask":
			host := cfg.WebHost
			if host == "0.0.0.0" {
				host = "127.0.0.1"
			}
			runCmd = fmt.Sprintf("FLASK_APP=%s %s -m flask run --host=%s --port=%d",
				cfg.EntryPoint, cfg.PythonCommand, host, cfg.WebPort)
		case "fastapi", "uvicorn":
			entry := strings.ReplaceAll(cfg.EntryPoint, ".py", "")
			if cfg.WebThreads > 0 {
				runCmd = fmt.Sprintf("%s -m uvicorn %s:app --host %s --port %d --workers %d --threads %d",
					cfg.PythonCommand, entry, cfg.WebHost, cfg.WebPort, cfg.WebWorkers, cfg.WebThreads)
			} else if cfg.WebWorkers > 0 {
				runCmd = fmt.Sprintf("%s -m uvicorn %s:app --host %s --port %d --workers %d",
					cfg.PythonCommand, entry, cfg.WebHost, cfg.WebPort, cfg.WebWorkers)
			} else {
				runCmd = fmt.Sprintf("%s -m uvicorn %s:app --host %s --port %d",
					cfg.PythonCommand, entry, cfg.WebHost, cfg.WebPort)
			}
		case "gunicorn":
			workers := cfg.WebWorkers
			if workers == 0 {
				workers = 4
			}
			entry := strings.ReplaceAll(cfg.EntryPoint, ".py", "")
			runCmd = fmt.Sprintf("%s -m gunicorn %s:app -w %d -b %s:%d",
				cfg.PythonCommand, entry, workers, cfg.WebHost, cfg.WebPort)
		default:
			if cfg.EntryPoint != "" {
				runCmd = fmt.Sprintf("%s %s", cfg.PythonCommand, cfg.EntryPoint)
			} else {
				runCmd = fmt.Sprintf("%s -m http.server %d", cfg.PythonCommand, cfg.WebPort)
			}
		}
	}

	runScript := "#!/bin/bash\n" + runCmd + "\n"
	scriptPath := filepath.Join(appDir, "run_server.sh")

	if err := os.WriteFile(scriptPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	if cfg.ServiceName == "" {
		return nil
	}

	// Generate systemd service file
	remoteAppDir := filepath.ToSlash(filepath.Join(ctx.Config.RemotePath, "current", "app", cfg.ProjectRoot))
	if cfg.ProjectRoot == "" {
		remoteAppDir = filepath.ToSlash(filepath.Join(ctx.Config.RemotePath, "current", "app"))
	}

	remoteScriptPath := filepath.ToSlash(filepath.Join(remoteAppDir, filepath.Base(scriptPath)))
	user := ctx.Config.SSH.User
	if user == "" {
		user = "root"
	}

	systemdContent := fmt.Sprintf(`[Unit]
Description=VersaDeploy Python Web Server (%s)
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=/bin/bash %s
Restart=always
RestartSec=3
Environment="PYTHONUNBUFFERED=1"

[Install]
WantedBy=multi-user.target
`, cfg.ServiceName, user, remoteAppDir, remoteScriptPath)

	servicePath := filepath.Join(appDir, cfg.ServiceName+".service")
	if err := os.WriteFile(servicePath, []byte(systemdContent), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	ctx.Log.Info("Generated systemd file: %s", cfg.ServiceName+".service")
	return nil
}
