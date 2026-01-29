package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/selfupdate"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/version"
)

var (
	configPath string
	verbose    bool
	debug      bool
	logFile    string
)

var rootCmd = &cobra.Command{
	Use:   "versa",
	Short: "versaDeploy - Production-grade deployment engine",
	Long: `versaDeploy is a deterministic deployment tool that:
- Detects changes via SHA256 hashing
- Builds artifacts selectively outside production
- Deploys atomically using symlink switching
- Supports instant rollback

Available Commands:
  deploy      Deploy to specified environment
  rollback    Rollback to previous release
  status      Show deployment status
  ssh-test    Test SSH connection to environment
  init        Initialize a new versaDeploy configuration
  version     Show application version
  self-update Check and install updates for versaDeploy`,
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

		// Initialize logger
		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()

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
		d, err := deployer.NewDeployer(cfg, env, repoPath, dryRun, initialDeploy, log)
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
		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, log)
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
		d, err := deployer.NewDeployer(cfg, env, repoPath, false, false, log)
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

		client, err := ssh.NewClient(&envCfg.SSH)
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
		if _, err := os.Stat("deploy.yml"); err == nil {
			return fmt.Errorf("deploy.yml already exists")
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
    
    # Timeout for each post_deploy hook in seconds (optional, default: 300)
    hook_timeout: 300
    
    # Files that trigger route cache regeneration
    route_files:
      - "app/routes.php"
    
    # Paths to ignore for SHA256 tracking
    ignored_paths:
      - ".git"
      - "tests"
      - "var/cache"
      - "node_modules/.cache"
    
    builds:
      php:
        enabled: true
        composer_command: "composer install --no-dev --optimize-autoloader"
      
      go:
        enabled: false
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "app"
      
      frontend:
        enabled: true
        npm_command: "npm ci" # Can be changed to "pnpm install" or "yarn install"
        compile_command: "npm run prod" # Use {file} if you want to compile files individually
    
    # Hooks to run on remote server after symlink switch
    post_deploy:
      - "php current/versa cache:clear"
      - "php current/versa routes:dump"
      - "php current/versa twig:clear-cache"
`
		err := os.WriteFile("deploy.yml", []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to create deploy.yml: %w", err)
		}

		fmt.Println("🚀 Initialized versaDeploy! Created deploy.yml.")
		fmt.Println("Edit deploy.yml to match your server details and then run: versa deploy production --initial-deploy")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "deploy.yml", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug mode")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log file path")

	deployCmd.Flags().Bool("dry-run", false, "Show changes without deploying")
	deployCmd.Flags().Bool("initial-deploy", false, "Flag for first deployment")

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
