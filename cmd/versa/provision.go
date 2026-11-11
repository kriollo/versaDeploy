package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/provision"
	"github.com/user/versaDeploy/internal/ssh"
)

var provisionCmd = &cobra.Command{
	Use:   "provision [environment]",
	Short: "Install / configure the server stack, as the Services view does (I, P, X)",
	Long: `Detects the server over provision.ssh (root) and installs what the Services view
preselects for the app: from deploy.yml's provision block and builds. It runs the same
steps as the view: idempotent, safe on a server in use (installed services keep their
config, an alternative is never installed next to another).

  --only      the steps to run instead of the preselection (versa provision ENV --steps lists them)
  --prepare   only the deploy user + key, its sudoers and the project directory (P)
  --reset-db-password   set the database user's password from db_password (X)`,
	Example: `  DB_PASS=… versa provision production
  versa provision production --dry-run
  versa provision production --only frankenphp,nginx,sudoers -y
  versa provision production --prepare`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envName := args[0]
		only, _ := cmd.Flags().GetStringSlice("only")
		prepare, _ := cmd.Flags().GetBool("prepare")
		resetPass, _ := cmd.Flags().GetBool("reset-db-password")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		yes, _ := cmd.Flags().GetBool("yes")
		listSteps, _ := cmd.Flags().GetBool("steps")
		if listSteps {
			for _, s := range provision.Catalog {
				fmt.Printf("  %-14s %s\n", s.ID, s.Label)
			}
			return nil
		}
		if n := btoi(len(only) > 0) + btoi(prepare) + btoi(resetPass); n > 1 {
			return fmt.Errorf("--only, --prepare and --reset-db-password can't be combined")
		}

		log, err := logger.NewLogger(logFile, verbose, debug)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer log.Close()
		if configPath, err = getOrSelectConfig(cmd); err != nil {
			return err
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		env, err := cfg.GetEnvironment(envName)
		if err != nil {
			return err
		}
		repoPath, err := os.Getwd()
		if err != nil {
			return err
		}

		admin := env.ProvisionSSH()
		client, err := ssh.NewClient(&admin, log)
		if err != nil {
			return fmt.Errorf("admin connection (provision.ssh): %w", err)
		}
		defer client.Close()
		r, err := provision.Detect(client, deployer.ProvisionDetectParams(env, cfg.Project))
		if err != nil {
			return err
		}
		_, v, ids := deployer.ProvisionDefaults(env, cfg.Project, repoPath, r)
		title := "Install / configure on " + envName
		switch {
		case prepare:
			ids, title = deployer.PrepareSteps, "Prepare "+envName+" for deploy (user "+v.DeployUser+")"
		case resetPass:
			if v.Database != "postgres" && v.Database != "mariadb" {
				return fmt.Errorf("--reset-db-password needs provision.database: postgres or mariadb")
			}
			v.ResetDBPassword = true
			ids, title = []string{v.Database}, "RESET password of DB user "+v.DBUser
		case len(only) > 0:
			if ids, err = selectSteps(&v, ids, only); err != nil {
				return err
			}
		}
		if len(ids) == 0 {
			fmt.Println("Nothing to install: the server already runs what the app needs (--only picks steps)")
			return nil
		}

		b, notes, err := deployer.ProvisionBundle(env, cfg.Project, envName, r, ids, v)
		if err != nil {
			return err
		}
		fmt.Printf("%s (%s@%s):\n", title, admin.User, admin.Host)
		for _, id := range ids {
			svc, _ := provision.Lookup(id)
			st := r.Get(id)
			fmt.Printf("  • %-34s %s\n", svc.Label, strings.TrimSpace(st.Status+" "+st.Version)) // e.g. absent (missing: zbar-tools)
		}
		for _, n := range notes {
			fmt.Println("  ⚠ " + n)
		}
		if resetPass {
			fmt.Println("  ⚠ Every app using " + v.DBUser + " must get the new password (.env)")
		}
		if dryRun {
			return nil
		}
		if !yes {
			fmt.Print("Proceed? [y/N]: ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				return fmt.Errorf("cancelled")
			}
		}
		return provision.Run(client, admin.User, b, os.Stdout)
	},
}

// selectSteps returns the steps in catalog order, applying to v what choosing each one
// means (as Space does in the view); preselected steps keep deploy.yml's parameters.
func selectSteps(v *provision.Vars, preselected, only []string) ([]string, error) {
	var out []string
	for _, s := range provision.Catalog {
		if !slices.Contains(only, s.ID) {
			continue
		}
		if s.ID == "app_service" {
			return nil, fmt.Errorf("app_service isn't a step: Python/Go/Node apps run as deploy.yml services, installed by the sudoers step")
		}
		if !slices.Contains(preselected, s.ID) {
			v.Select(s.ID)
		}
		out = append(out, s.ID)
	}
	for _, id := range only {
		if _, ok := provision.Lookup(id); !ok {
			return nil, fmt.Errorf("unknown step %q (versa provision ENV --steps lists them)", id)
		}
	}
	return out, nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func init() {
	provisionCmd.Flags().StringSlice("only", nil, "Run these steps instead of the preselection (comma-separated ids)")
	provisionCmd.Flags().Bool("prepare", false, "Only the deploy user, its sudoers and the project directory")
	provisionCmd.Flags().Bool("reset-db-password", false, "Set the database user's password from provision.db_password")
	provisionCmd.Flags().Bool("dry-run", false, "Show the steps without running them")
	provisionCmd.Flags().BoolP("yes", "y", false, "Don't ask for confirmation")
	provisionCmd.Flags().Bool("steps", false, "List the step ids and exit")
	rootCmd.AddCommand(provisionCmd)
}
