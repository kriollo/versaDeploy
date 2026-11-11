package deployer

import (
	"fmt"
	"os"
	"slices"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/provision"
)

// Server provisioning shared by the TUI's Services view (I, P, X) and `versa provision`:
// both detect, preselect and build the same bundle.

// PrepareSteps are what P (and `versa provision --prepare`) run: the deploy user and
// its key, its sudoers and the project directory.
var PrepareSteps = []string{"deploy_user", "sudoers", "dirs"}

// ProvisionDetectParams is what the detection script needs from the environment.
func ProvisionDetectParams(env *config.Environment, project string) provision.DetectParams {
	p := provision.DetectParams{Project: project, RemotePath: env.RemotePath, DeployUser: env.Provision.DeployUser,
		PHPVersion: env.Provision.PHPVersion, NodeMajor: env.Provision.NodeMajor, Packages: env.Provision.Packages}
	if len(env.Services) > 0 {
		p.AppService = env.Services[0].Name
	}
	if p.DeployUser == "" {
		p.DeployUser = env.SSH.User
	}
	return p
}

// ProvisionDefaults returns the app's backends, the parameters (deploy.yml provision +
// detection, with the public key of ssh.key_path) and the preselected steps.
func ProvisionDefaults(env *config.Environment, project, repoPath string, r *provision.Report) ([]provision.Backend, provision.Vars, []string) {
	backends := provision.DetectBackends(env, repoPath)
	v := provision.Defaults(env, project, backends, r)
	if pub, err := os.ReadFile(env.SSH.KeyPath + ".pub"); err == nil {
		v.PubKey = string(pub)
	}
	return backends, v, provision.Preselect(backends, v, env, r)
}

// ProvisionBundle checks the steps against the server and returns the bundle to run as
// root, with the notes to show before confirming. With the sudoers step it also installs
// the environment's services and their rules, run by the deploy user.
func ProvisionBundle(env *config.Environment, project, envName string, r *provision.Report, ids []string, v provision.Vars) (provision.Bundle, []string, error) {
	if !r.CanInstall() {
		return provision.Bundle{}, nil, fmt.Errorf("install needs an apt (Debian/Ubuntu) or dnf (RHEL/Rocky/Alma 8+) server")
	}
	if err := provision.Conflicts(r, ids, v); err != nil {
		return provision.Bundle{}, nil, err
	}
	if env != nil && slices.Contains(ids, "sudoers") {
		e := *env
		svcs, err := ServicesForDeployUser(env, v)
		if err != nil {
			return provision.Bundle{}, nil, err
		}
		e.Services = svcs
		if len(e.Services) > 0 {
			e.SSH.User = v.DeployUser
			v.ServicesSudoers = SudoersScript(&e, project)
			if v.ServicesInstall, err = ServiceInstallScript(&e, project, envName); err != nil {
				return provision.Bundle{}, nil, err
			}
		}
	}
	b, err := provision.Build(ids, v)
	if err != nil {
		return provision.Bundle{}, nil, err
	}
	var notes []string
	if r.ProjectSite != "" && (slices.Contains(ids, "nginx") || slices.Contains(ids, "apache")) {
		notes = append(notes, "This app is already served by "+r.ProjectSite+": the install stops if it",
			"serves the same domain (it is never overwritten)")
	}
	if slices.Contains(ids, "ufw") {
		notes = append(notes, fmt.Sprintf("UFW: allows SSH %d, 80 and 443 (an active firewall keeps its policy)", v.SSHPort))
	}
	return b, notes, nil
}

// ServicesForDeployUser returns the services the sudoers step installs for the deploy
// user. The rules go to that user, so services that ran as the old ssh.user run as
// the deploy user, and none may run as root: the deploy user owns the wrapper root
// would execute.
func ServicesForDeployUser(env *config.Environment, v provision.Vars) ([]config.ServiceConfig, error) {
	svcs := slices.Clone(provision.ProposedServices(env, v))
	for i := range svcs {
		if u := svcs[i].User; u == "" || u == env.SSH.User {
			svcs[i].User = v.DeployUser
		}
		if svcs[i].User == "root" && v.DeployUser != "root" {
			return nil, fmt.Errorf("service %s runs as root: set its user to %s", svcs[i].Name, v.DeployUser)
		}
	}
	return svcs, nil
}
