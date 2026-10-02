package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/selfupdate"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/tui"
	"github.com/user/versaDeploy/internal/version"
)

var (
	configPath string
	verbose    bool
	debug      bool
	logFile    string
	guiMode    bool
	noGUI      bool
)

var rootCmd = &cobra.Command{
	Use:     "versa",
	Short:   "versaDeploy - Production-grade deployment engine",
	Version: version.Version,
	Long: `versaDeploy is a deterministic deployment tool that:
- Detects changes via SHA256 hashing
- Builds artifacts selectively outside production
- Deploys atomically using symlink switching
- Supports instant rollback`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if noGUI {
			return cmd.Help()
		}

		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		var cfg *config.Config
		// If user explicitly provided --config, we MUST try to load it.
		if cmd.Flags().Changed("config") {
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load specified config: %w", err)
			}
		} else {
			// Try default, but don't fail hard if it's missing (TUI will discover others)
			cfg, _ = config.Load(configPath)
		}

		return tui.Launch(cfg, repoPath)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show application version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("versaDeploy %s\n", version.Version)
	},
}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Check and install updates for versaDeploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return err
		}
		defer log.Close()

		updater := selfupdate.NewUpdater(log)
		return updater.Update()
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy [environment]",
	Short: "Deploy to specified environment (comma-separated list for multiple: env1,env2)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envNames := strings.Split(args[0], ",")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		initialDeploy, _ := cmd.Flags().GetBool("initial-deploy")
		force, _ := cmd.Flags().GetBool("force")
		skipDirtyCheck, _ := cmd.Flags().GetBool("skip-dirty-check")

		// Initialize logger
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		// Determine configuration file
		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get current working directory as repository path
		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		for i, env := range envNames {
			env = strings.TrimSpace(env)
			if len(envNames) > 1 {
				fmt.Printf("\n=== Deploying %s (%d/%d) ===\n", env, i+1, len(envNames))
			}

			// Create deployer
			d, err := deployer.NewDeployer(cfg, env, repoPath, dryRun, initialDeploy, force, skipDirtyCheck, log)
			if err != nil {
				return err
			}

			// On initial deploy, confirm before running post_deploy hooks
			if initialDeploy {
				d.PostDeployConfirm = func() bool {
					fmt.Println()
					fmt.Println("  ⚠  INITIAL DEPLOY — post_deploy hooks are about to run.")
					fmt.Println("     Make sure your configuration file and .env are correctly")
					fmt.Println("     set up on the server before proceeding.")
					fmt.Print("     Run post_deploy hooks? [y/N]: ")
					var answer string
					fmt.Scanln(&answer)
					return strings.ToLower(strings.TrimSpace(answer)) == "y"
				}
			}

			// Execute deployment
			if err := d.Deploy(); err != nil {
				if len(envNames) > 1 {
					return fmt.Errorf("deploy failed for %s: %w", env, err)
				}
				return err
			}
		}

		return nil
	},
}

var diffCmd = &cobra.Command{
	Use:   "diff [environment]",
	Short: "List the files a deploy would ship (committed HEAD vs the server's deploy.lock)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if env, err := cfg.GetEnvironment(args[0]); err != nil {
			return err
		} else if env.Local {
			return fmt.Errorf("diff needs an SSH environment (local deploys have no server state)")
		}

		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		// Dry run of a normal deploy; only HEAD is compared, so uncommitted changes don't matter
		d, err := deployer.NewDeployer(cfg, args[0], repoPath, true, false, false, true, log)
		if err != nil {
			return err
		}
		return d.Deploy()
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback [environment]",
	Short: "Rollback to previous release (or specific version with --to)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]
		targetVersion, _ := cmd.Flags().GetString("to")

		// Initialize logger
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		// Determine configuration file
		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get current working directory
		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create deployer
		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}

		// Execute rollback
		if targetVersion != "" {
			return d.RollbackTo(targetVersion)
		}
		return d.Rollback()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [environment]",
	Short: "Show deployment status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]

		// Initialize logger
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		// Determine configuration file
		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get current working directory
		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create deployer
		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}

		// Show status
		return d.Status()
	},
}

var sshTestCmd = &cobra.Command{
	Use:     "ssh-test [environment]",
	Aliases: []string{"info"},
	Short:   "Test SSH connection and show server info (OS, kernel, resources, runtimes)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]

		// Initialize logger
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		// Determine configuration file
		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Find environment config
		envCfg, err := cfg.GetEnvironment(env)
		if err != nil {
			return err
		}

		fmt.Printf("🔍 Testing SSH connection to %s (%s)...\n", env, envCfg.SSH.User+"@"+envCfg.SSH.Host)

		client, err := ssh.NewClient(&envCfg.SSH, log)
		if err != nil {
			return fmt.Errorf("❌ SSH connection failed: %w", err)
		}
		defer client.Close()

		fmt.Println("✅ SSH connection established successfully!")

		fmt.Println("🔍 Probing server...")
		info, err := client.ServerInfo(envCfg.RemotePath, true)
		if err != nil {
			return fmt.Errorf("❌ remote command execution failed: %w", err)
		}
		printServerInfo(info)

		// Test SFTP
		fmt.Println("🔍 Testing SFTP subsystem...")
		exists, err := client.FileExists(".")
		if err != nil {
			return fmt.Errorf("❌ SFTP test failed: %w", err)
		}
		if exists {
			fmt.Println("✅ SFTP subsystem working.")
		}

		if len(envCfg.Services) > 0 {
			fmt.Println("🔍 Checking sudo for services...")
			missing, err := deployer.ServiceSudoCheck(client, envCfg)
			switch {
			case err != nil:
				fmt.Printf("⚠️  Could not check sudo: %v\n", err)
			case len(missing) > 0:
				fmt.Printf("⚠️  %s can't restart %s with passwordless sudo: deploys will fail at the service step.\n", envCfg.SSH.User, strings.Join(missing, ", "))
				fmt.Printf("   Run 'versa service %s sudoers' and add the printed line on the server.\n", env)
			default:
				fmt.Println("✅ Services can be managed with sudo.")
			}
		}

		fmt.Println("\n✨ SSH connection test passed!")
		return nil
	},
}

func printServerInfo(i *ssh.ServerInfo) {
	cpu := i.CPUModel
	if i.Cores != "" {
		cpu += " (" + i.Cores + " cores)"
	}
	rows := [][2]string{
		{"Host", i.Hostname}, {"OS", i.OS}, {"Kernel", strings.TrimSpace(i.Kernel + " " + i.Arch)},
		{"Init / libc", strings.Trim(i.Init+" / "+i.Libc, " /")}, {"CPU", cpu}, {"CPU usage", i.CPU},
		{"RAM", i.RAM}, {"Swap", i.Swap}, {"Load", i.Load}, {"Uptime", i.Uptime},
		{"Disk (deploy)", i.Disk}, {"Inodes (deploy)", i.Inodes},
	}
	for _, rt := range []string{"php", "node", "python", "git"} {
		rows = append(rows, [2]string{rt, i.Runtimes[rt]})
	}
	rows = append(rows, [2]string{"tools", strings.Join(i.Tools, " ")})
	for _, r := range rows {
		if r[1] == "" {
			r[1] = "—"
		}
		fmt.Printf("   %-16s %s\n", r[0]+":", r[1])
	}
	if i.Sysname != "" && !i.Has("tar") {
		fmt.Println("⚠️  tar not found: deploys will fail until it is installed")
	}
}

// buildsTemplate is shared by both the SSH (VPS) and local init templates —
// which language builders are available doesn't depend on how the result gets
// to the server.
const buildsTemplate = `    builds:
      php:
        enabled: false
        composer_command: "composer install --no-dev --optimize-autoloader"

      go:
        enabled: false
        root: ""                       # Subdirectory where your go.mod lives (if any)
        deploy_path: "bin/go"          # Release-relative output path for the Go binary
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "app"

      frontend:
        enabled: false
        npm_command: "npm ci" # Can be changed to "pnpm install" or "yarn install"
        compile_command: "npm run prod"

      python:
        enabled: true

        # --- Basic settings ---
        # Dependencies are installed on the SERVER, in a virtualenv inside each release
        # (reused when requirements don't change). The server needs python3 + venv.
        root: ""                          # Subdirectory where your Python project lives (if any)
        python_command: "python3"         # Python on the server
        package_manager: "pip"            # pip (default), poetry, pipenv
        requirements_file: "requirements.txt"
        venv_path: ".venv"

        # --- Web server mode ---
        # Enable if deploying a web app (Flask, FastAPI, Django, etc.)
        web_server: false
        web_framework: "fastapi"          # django, flask, fastapi, uvicorn, gunicorn
        entry_point: "main.py"            # App entry point
        web_host: "0.0.0.0"
        web_port: 8000
        web_workers: 2                    # Number of worker processes
        # web_threads: 0                  # Threads per worker (uvicorn only)

        # Custom run command (overrides web_framework auto-detection)
        # run_command: "python3 -m uvicorn main:app --host 0.0.0.0 --port 8000"

        # --- Binary build (PyInstaller) ---
        # Compiles a standalone executable (no Python needed on server).
        # PyInstaller can't cross-compile: only works when you run versa on Linux.
        build_binary: false
        # entry_point: "main.py"          # Required when build_binary: true
        # binary_name: "myapp"

        # --- WebSocket support ---
        websocket: false
        # ws_protocol: "channels"         # websocket, socket.io, channels (Django)

        # --- Dependency options ---
        install_dev_deps: false
        use_cache: false
        # pypi_mirror: ""                 # Custom PyPI mirror URL
        # torch_index: ""                 # PyTorch index (e.g. https://download.pytorch.org/whl/cpu)
        # extra_requirements: []          # Extra requirements files to install

        # Reuse .venv from previous release (speeds up deploys)
        reusable_paths:
          - ".venv"
`

var sshConfigTemplate = `project: "my-versa-project"

environments:
  production:
    ssh:
      host: "server.example.com"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"
      port: 22
      known_hosts_file: "~/.ssh/known_hosts"
      use_ssh_agent: false

    remote_path: "/var/www/app"

    # Timeout for each hook in seconds (optional, default: 300)
    hook_timeout: 300

    # Paths to ignore for SHA256 tracking
    ignored_paths:
      - ".git"
      - "tests"
      - "var/cache"
      - "node_modules/.cache"

    # Paths that persist between releases (symlinked into each release)
    shared_paths:
      - ".env"
      # - "storage/logs"
      # - "public/uploads"

` + buildsTemplate + `
    # Hooks to run locally before cloning (abort on failure)
    # pre_deploy_local:
    #   - "make test"
    #   - "go vet ./..."

    # Hooks to run on remote server before symlink switch (non-fatal warnings)
    # pre_deploy_server:
    #   - "sudo systemctl stop myapp || true"

    # Hooks to run on remote server after symlink switch (rollback on failure)
    post_deploy: []

    # Long-running processes (Go binary, Python server). versa installs them in the
    # server's init system (systemd / OpenRC / SysV), starts them at boot and restarts
    # them on every deploy and rollback. Run 'versa service production sudoers' to get
    # the sudoers line the SSH user needs.
    # services:
    #   - name: "myapp"
    #     # exec: "./bin/go/app --port 8080"   # default: the go binary or python's run_server.sh
    #     env_file: ".env"                     # loaded from <remote_path>/shared/.env
`

var localConfigTemplate = `project: "my-versa-project"

environments:
  local:
    # Local mode: no SSH. versa builds the project and writes a full release
    # to local_path, replacing it entirely on every deploy — upload the
    # contents of local_path/app to your hosting (FTP, file manager, etc.).
    # There is no release history in this mode, so shared_paths, preserved_paths
    # and pre_deploy_server are not available (they only make sense with the
    # symlink-based releases the SSH mode manages).
    local: true
    local_path: "./local-releases/local" # never point this at a build tool's own output dir (e.g. frontend's "dist")

    # Timeout for hooks in seconds (optional, default: 300)
    hook_timeout: 300

    # Paths to ignore for SHA256 tracking
    ignored_paths:
      - ".git"
      - "tests"
      - "var/cache"
      - "node_modules/.cache"

` + buildsTemplate + `
    # Hooks to run locally before building (abort on failure)
    # pre_deploy_local:
    #   - "make test"

    # Hooks to run locally after the release is written to local_path
    # (cwd = local_path/app)
    # post_deploy:
    #   - "php versaCLI cache:clear"
`

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new versaDeploy configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("%s already exists", configPath)
		}

		local := initLocal
		if !cmd.Flags().Changed("local") {
			fmt.Println("¿Cómo vas a desplegar este proyecto?")
			fmt.Println("  [1] VPS / servidor con acceso SSH (por defecto)")
			fmt.Println("  [2] Hosting local sin SSH (genera una carpeta para subir manualmente)")
			fmt.Print("Elige una opción [1/2]: ")
			var answer string
			fmt.Scanln(&answer)
			local = strings.TrimSpace(answer) == "2"
		}

		content := sshConfigTemplate
		if local {
			content = localConfigTemplate
		}

		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create %s: %w", configPath, err)
		}

		fmt.Printf("🚀 Initialized versaDeploy! Created %s.\n", configPath)
		if local {
			fmt.Printf("Edit %s to match your build settings and then run: versa deploy local\n", configPath)
		} else {
			fmt.Printf("Edit %s to match your server details and then run: versa deploy production --initial-deploy\n", configPath)
		}
		return nil
	},
}

var execCmd = &cobra.Command{
	Use:   "exec [environment] [command]",
	Short: "Execute a command on the remote server",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]
		remoteCmd := strings.Join(args[1:], " ")

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}

		output, err := d.ExecRemoteCommand(remoteCmd)
		if output != "" {
			fmt.Print(output)
		}
		return err
	},
}

var hooksCmd = &cobra.Command{
	Use:   "hooks [environment] [indices...]",
	Short: "Re-execute post_deploy hooks on the active release",
	Long:  "Re-execute all post_deploy hooks, or specific ones by index (0-based). Example: versa hooks production 0 2",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}

		// Parse indices if provided
		var indices []int
		for _, arg := range args[1:] {
			idx, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid hook index %q: must be a number", arg)
			}
			indices = append(indices, idx)
		}

		return d.RunHooks(indices)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [environment] [path]",
	Short: "Tail remote log files in real-time",
	Long:  "Stream remote log files using tail -f. Default: follows the most common Laravel log. Example: versa logs production /var/log/syslog",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]
		lines, _ := cmd.Flags().GetInt("lines")

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		envCfg, err := cfg.GetEnvironment(env)
		if err != nil {
			return err
		}

		// Determine log path
		logPath := ""
		if len(args) > 1 {
			logPath = args[1]
		} else {
			// Default: Laravel storage/logs/laravel.log via current symlink
			logPath = filepath.ToSlash(filepath.Join(envCfg.RemotePath, "current", "app", "storage", "logs", "laravel.log"))
		}

		tailCmd := fmt.Sprintf("tail -n %d -f %s", lines, logPath)

		sshClient, err := ssh.NewClient(&envCfg.SSH, log)
		if err != nil {
			return err
		}
		defer sshClient.Close()

		fmt.Printf("Tailing %s (Ctrl+C to stop)...\n", logPath)
		return sshClient.ExecuteCommandStreaming(tailCmd, os.Stdout, os.Stderr)
	},
}

var servicesReloadCmd = &cobra.Command{
	Use:   "services-reload [environment]",
	Short: "Run services_reload commands (e.g. reload php-fpm/nginx) without a full deploy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}

		return d.ReloadServices()
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service [environment] [action]",
	Short: "Manage the environment's services (status, start, stop, restart, logs, install, uninstall, sudoers)",
	Long: `Manage the long-running processes listed under 'services' in the config. They run
through the server's init system (systemd, OpenRC or SysV init), start at boot and are
restarted on every deploy and rollback.

Actions:
  status     show whether each service is running (default)
  start      start, then check it stays up
  stop       stop
  restart    restart, then check it stays up
  logs       follow the log (journalctl on systemd, /var/log/<name>.log otherwise)
  install    (re)install and enable at boot, without deploying
  uninstall  stop, disable and remove from the init system
  sudoers    print the sudoers line the deploy user needs on this server`,
	Example: `  versa service production
  versa service production restart
  versa service production logs --name api --lines 200
  versa service production sudoers`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := "status"
		if len(args) == 2 {
			action = args[1]
		}
		name, _ := cmd.Flags().GetString("name")
		lines, _ := cmd.Flags().GetInt("lines")

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		repoPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		d, err := deployer.NewDeployer(cfg, args[0], repoPath, false, false, false, false, log)
		if err != nil {
			return err
		}
		return d.ServiceAction(action, name, lines, os.Stdout)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and validate deploy.yml configuration",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate [environment]",
	Short: "Validate deploy.yml without connecting to any server",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := getOrSelectConfig(cmd)
		if err != nil {
			return err
		}
		configPath = path

		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		names := make([]string, 0, len(cfg.Environments))
		for n := range cfg.Environments {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Printf("✅ %s is valid (%d environment(s): %s)\n", configPath, len(names), strings.Join(names, ", "))

		if len(args) == 1 {
			envCfg, err := cfg.GetEnvironment(args[0])
			if err != nil {
				return err
			}
			printEnvSummary(args[0], envCfg)
		}

		return nil
	},
}

func printEnvSummary(name string, e *config.Environment) {
	fmt.Printf("\nEnvironment %q:\n", name)
	if e.Local {
		fmt.Printf("  mode:       local\n")
		fmt.Printf("  local_path: %s\n", e.LocalPath)
	} else {
		fmt.Printf("  mode:        ssh\n")
		fmt.Printf("  ssh:         %s@%s:%d\n", e.SSH.User, e.SSH.Host, e.SSH.Port)
		fmt.Printf("  remote_path: %s\n", e.RemotePath)
	}

	var builds []string
	if e.Builds.PHP.Enabled {
		builds = append(builds, "php")
	}
	if e.Builds.Go.Enabled {
		builds = append(builds, "go")
	}
	if e.Builds.Frontend.Enabled {
		builds = append(builds, "frontend")
	}
	if e.Builds.Python.Enabled {
		builds = append(builds, "python")
	}
	fmt.Printf("  builds:      %s\n", strings.Join(builds, ", "))
	fmt.Printf("  hooks:       pre_deploy_local=%d pre_deploy_server=%d post_deploy=%d\n",
		len(e.PreDeployLocal), len(e.PreDeployServer), len(e.PostDeploy))
	for _, s := range e.Services {
		exec := s.Exec
		if exec == "" {
			exec, _ = e.DefaultServiceExec(e.RemotePath + "/current")
		}
		fmt.Printf("  service:     %s (user %s): %s\n", s.Name, s.User, exec)
	}
}

func getOrSelectConfig(cmd *cobra.Command) (string, error) {
	// If the user explicitly provided a config flag, use it
	if cmd.Flags().Changed("config") {
		return configPath, nil
	}

	// If GUI mode is enabled, we don't want to prompt in CLI.
	// We'll let the TUI handle it later.
	if guiMode {
		return configPath, nil
	}

	// Try to discover config files automatically
	cwd, err := os.Getwd()
	if err != nil {
		return configPath, nil
	}

	files, err := config.FindConfigFiles(cwd)
	if err != nil || len(files) == 0 {
		// fallback to original default
		return configPath, nil
	}

	if len(files) == 1 {
		return files[0], nil
	}

	// If there are multiple configuration files, prompt the user
	fmt.Println("\nMultiple configuration files found. Please select one:")
	for i, f := range files {
		fmt.Printf("[%d] %s\n", i+1, filepath.Base(f))
	}
	fmt.Print("Enter number: ")

	var input string
	_, err = fmt.Scanln(&input)
	if err != nil {
		// Just fallback if scanning fails
		return configPath, nil
	}

	input = strings.TrimSpace(input)
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(files) {
		return "", fmt.Errorf("invalid selection")
	}

	fmt.Println()
	return files[idx-1], nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "deploy.yml", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug mode")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log file path")
	rootCmd.PersistentFlags().BoolVar(&guiMode, "gui", false, "Launch interactive TUI (default behavior; kept for backward compat)")
	rootCmd.PersistentFlags().BoolVar(&noGUI, "no-gui", false, "Disable TUI and show help")

	deployCmd.Flags().Bool("dry-run", false, "Show changes without deploying")
	deployCmd.Flags().Bool("initial-deploy", false, "Flag for first deployment")
	deployCmd.Flags().Bool("force", false, "Force redeploy even if no changes detected")
	deployCmd.Flags().Bool("skip-dirty-check", false, "Skip validation of uncommitted changes")

	rollbackCmd.Flags().String("to", "", "Rollback to a specific release version (e.g. 20240101_120000)")

	logsCmd.Flags().Int("lines", 50, "Number of initial lines to show before following")

	serviceCmd.Flags().String("name", "", "Act on one service only (default: all; required for logs with several services)")
	serviceCmd.Flags().Int("lines", 50, "logs: number of initial lines to show before following")

	initCmd.Flags().BoolVar(&initLocal, "local", false, "Generate a local (no SSH) environment instead of prompting")

	configCmd.AddCommand(configValidateCmd)

	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sshTestCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(selfUpdateCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(hooksCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(servicesReloadCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	// main prints errors itself (formatted); usage is only useful for bad arguments, and
	// cobra validates those before PersistentPreRun runs
	rootCmd.SilenceErrors = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true }
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, verserrors.FormatError(verserrors.Wrap(err)))
		os.Exit(1)
	}
}
