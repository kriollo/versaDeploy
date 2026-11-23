package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/provision"
	"github.com/user/versaDeploy/internal/ssh"
)

// adminSession connects over provision.ssh and resolves the install vars.
func adminSession(cmd *cobra.Command, envName string) (*ssh.Client, string, provision.Vars, func(), error) {
	log, err := logger.NewLogger(logFile, verbose, debug)
	if err != nil {
		return nil, "", provision.Vars{}, nil, err
	}
	if configPath, err = getOrSelectConfig(cmd); err != nil {
		log.Close()
		return nil, "", provision.Vars{}, nil, err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Close()
		return nil, "", provision.Vars{}, nil, fmt.Errorf("failed to load config: %w", err)
	}
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		log.Close()
		return nil, "", provision.Vars{}, nil, err
	}
	repo, err := os.Getwd()
	if err != nil {
		log.Close()
		return nil, "", provision.Vars{}, nil, err
	}
	admin := env.ProvisionSSH()
	client, err := ssh.NewClient(&admin, log)
	if err != nil {
		log.Close()
		return nil, "", provision.Vars{}, nil, fmt.Errorf("admin connection (provision.ssh): %w", err)
	}
	done := func() { client.Close(); log.Close() }
	r, err := provision.Detect(client, deployer.ProvisionDetectParams(env, cfg.Project))
	if err != nil {
		done()
		return nil, "", provision.Vars{}, nil, err
	}
	_, v, _ := deployer.ProvisionDefaults(env, cfg.Project, repo, r)
	return client, admin.User, v, done, nil
}

var tuneCmd = &cobra.Command{
	Use:   "tune [environment]",
	Short: "Show or apply provision.tuning, sized for the server's CPU and RAM",
	Long: `Without flags it only prints the plan, by how each change takes effect:
  [live] now   [reload] no dropped requests   [restart] needs --restart   [others] affects other apps

--apply writes the project's own config and applies what is safe. Nothing restarts
unless named one by one: --restart anima,postgresql.`,
	Example: "  versa tune production\n  versa tune production --apply\n  versa tune production --apply --restart anima",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apply, _ := cmd.Flags().GetBool("apply")
		restart, _ := cmd.Flags().GetStringSlice("restart")
		if len(restart) > 0 && !apply {
			return fmt.Errorf("--restart needs --apply")
		}
		client, user, v, done, err := adminSession(cmd, args[0])
		if err != nil {
			return err
		}
		defer done()
		plan := provision.TunePlan(v)
		if len(plan) == 0 {
			fmt.Println("provision.tuning.profile is off: nothing to do")
			return nil
		}
		fmt.Println(strings.Join(plan, "\n"))
		if !apply {
			fmt.Println("\nNothing changed. Apply with --apply.")
			return nil
		}
		b, err := provision.TuneBundle(v, restart)
		if err != nil {
			return err
		}
		return provision.Run(client, user, b, os.Stdout)
	},
}

var dbCmd = &cobra.Command{Use: "db", Short: "Database operations on the server"}

var dbRestoreCmd = &cobra.Command{
	Use:   "restore [environment] [backup file]",
	Short: "Prove a backup restores, into a temporary database (never the live one)",
	Long: `Without a file it lists the backups. With one, it restores it into a temporary
database, compares row counts per table with the live database and drops it (--keep
leaves it there to inspect).`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		keep, _ := cmd.Flags().GetBool("keep")
		client, user, v, done, err := adminSession(cmd, args[0])
		if err != nil {
			return err
		}
		defer done()
		file := ""
		if len(args) == 2 {
			file = args[1]
		}
		script, err := provision.RestoreScript(v, file, keep)
		if err != nil {
			return err
		}
		return provision.Exec(client, user, script, os.Stdout)
	},
}

func init() {
	tuneCmd.Flags().Bool("apply", false, "Apply the plan")
	tuneCmd.Flags().StringSlice("restart", nil, "Services to restart: postgresql, mariadb or the app's service")
	dbRestoreCmd.Flags().Bool("keep", false, "Keep the temporary database")
	dbCmd.AddCommand(dbRestoreCmd)
	rootCmd.AddCommand(tuneCmd, dbCmd)
}
