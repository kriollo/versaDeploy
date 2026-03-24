package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	Short: "Deploy to specified environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]
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

		// Create deployer
		d, err := deployer.NewDeployer(cfg, env, repoPath, dryRun, initialDeploy, force, skipDirtyCheck, log)
		if err != nil {
			return err
		}

		// Execute deployment
		return d.Deploy()
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback [environment]",
	Short: "Rollback to previous release",
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

		// Execute rollback
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
	Use:   "ssh-test [environment]",
	Short: "Test SSH connection to specified environment",
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

		// Test command execution
		fmt.Println("🔍 Testing command execution...")
		output, err := client.ExecuteCommand("uname -a")
		if err != nil {
			// Fallback for Windows or systems without uname
			output, _ = client.ExecuteCommand("whoami")
		}
		if output != "" {
			fmt.Printf("✅ Remote system response: %s", output)
		}

		// Test SFTP
		fmt.Println("🔍 Testing SFTP subsystem...")
		exists, err := client.FileExists(".")
		if err != nil {
			return fmt.Errorf("❌ SFTP test failed: %w", err)
		}
		if exists {
			fmt.Println("✅ SFTP subsystem working.")
		}

		fmt.Println("\n✨ SSH connection test passed!")
		return nil
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new versaDeploy configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("%s already exists", configPath)
		}

		content := `project: "my-versa-project"

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

    builds:
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
        root: ""                          # Subdirectory where your Python project lives (if any)
        python_command: "python3"
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

        # --- systemd service management ---
        # If set, generates a .service file ready to install on the server.
        # First deploy: sudo cp /var/www/app/current/app/<name>.service /etc/systemd/system/
        #               sudo systemctl enable <name> && sudo systemctl start <name>
        service_name: ""                  # e.g. "myapp"

        # --- Binary build (PyInstaller) ---
        # Compiles a standalone executable (no Python needed on server)
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

    # Hooks to run locally before cloning (abort on failure)
    # pre_deploy_local:
    #   - "make test"
    #   - "go vet ./..."

    # Hooks to run on remote server before symlink switch (non-fatal warnings)
    # pre_deploy_server:
    #   - "sudo systemctl stop myapp || true"

    # Hooks to run on remote server after symlink switch (rollback on failure)
    post_deploy:
      # Restart systemd service after each deploy (requires service_name to be set above)
      # - "sudo systemctl restart myapp"
      []
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", configPath, err)
		}

		fmt.Printf("🚀 Initialized versaDeploy! Created %s.\n", configPath)
		fmt.Printf("Edit %s to match your server details and then run: versa deploy production --initial-deploy\n", configPath)
		return nil
	},
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

	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sshTestCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(selfUpdateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, verserrors.FormatError(verserrors.Wrap(err)))
		os.Exit(1)
	}
}
