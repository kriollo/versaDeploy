package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/logger"
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
- Supports instant rollback`,
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, verserrors.FormatError(verserrors.Wrap(err)))
		os.Exit(1)
	}
}
