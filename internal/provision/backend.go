package provision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/versaDeploy/internal/config"
)

// Backend is an application runtime the repo needs on the server.
type Backend struct {
	Kind   string // php | python | go | node
	Detail string // e.g. "php >=8.3", "fastapi :8000"
	Source string // deploy.yml or the file it came from
}

// DetectBackends finds the app's backends: the deploy.yml builds win; without builds
// the repo files decide. Node counts only as a server (package.json "start" script);
// a frontend that is only compiled ships as static files.
func DetectBackends(env *config.Environment, repoPath string) []Backend {
	var out []Backend
	b := env.Builds
	if b.PHP.Enabled {
		out = append(out, Backend{"php", phpConstraint(filepath.Join(repoPath, b.PHP.ProjectRoot)), "deploy.yml"})
	}
	if b.Python.Enabled {
		d := b.Python.WebFramework
		if b.Python.WebServer && b.Python.WebPort > 0 {
			d = strings.TrimSpace(d + " :" + strconv.Itoa(b.Python.WebPort))
		}
		out = append(out, Backend{"python", d, "deploy.yml"})
	}
	if b.Go.Enabled {
		out = append(out, Backend{"go", b.Go.BinaryName, "deploy.yml"})
	}
	if b.Frontend.Enabled && !b.PHP.Enabled {
		if pkg, ok := readPackageJSON(filepath.Join(repoPath, b.Frontend.ProjectRoot)); ok && pkg.Scripts["start"] != "" {
			out = append(out, Backend{"node", pkg.Engines["node"], "deploy.yml"})
		}
	}
	if len(out) > 0 {
		return out
	}

	exists := func(name string) bool { _, err := os.Stat(filepath.Join(repoPath, name)); return err == nil }
	if exists("composer.json") {
		out = append(out, Backend{"php", phpConstraint(repoPath), "composer.json"})
	}
	for _, f := range []string{"pyproject.toml", "requirements.txt", "Pipfile"} {
		if exists(f) {
			out = append(out, Backend{"python", "", f})
			break
		}
	}
	if exists("go.mod") {
		out = append(out, Backend{"go", "", "go.mod"})
	}
	if pkg, ok := readPackageJSON(repoPath); ok && pkg.Scripts["start"] != "" && !exists("composer.json") {
		out = append(out, Backend{"node", pkg.Engines["node"], "package.json"})
	}
	return out
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
	Engines map[string]string `json:"engines"`
}

func readPackageJSON(dir string) (packageJSON, bool) {
	var p packageJSON
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil || json.Unmarshal(data, &p) != nil {
		return p, false
	}
	return p, true
}

func phpConstraint(dir string) string {
	var c struct {
		Require map[string]string `json:"require"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "composer.json"))
	if err != nil || json.Unmarshal(data, &c) != nil || c.Require["php"] == "" {
		return ""
	}
	return "php " + c.Require["php"]
}

var majorRe = regexp.MustCompile(`\d+`)

// Defaults builds the install vars from the environment's provision block, falling back
// to what the server and the repo already tell us.
func Defaults(env *config.Environment, project string, backends []Backend, r *Report) Vars {
	p := env.Provision
	v := Vars{
		Project:        project,
		Domain:         p.Domain,
		WebServer:      p.WebServer,
		PHPVersion:     p.PHPVersion,
		RemotePath:     env.RemotePath,
		DocRoot:        p.DocRoot,
		DeployUser:     p.DeployUser,
		Database:       p.Database,
		DBName:         p.DBName,
		DBUser:         p.DBUser,
		DBPassword:     p.DBPassword,
		AdminEmail:     p.AdminEmail,
		SSHPort:        env.ProvisionSSH().Port,
		ProxyPort:      p.ProxyPort,
		NodeMajor:      p.NodeMajor,
		PythonVersion:  p.PythonVersion,
		Redis:          p.Redis,
		Dragonfly:      p.Dragonfly,
		FrankenPHP:     p.FrankenPHP,
		Aliases:        p.Aliases,
		BackupKeep:     p.BackupKeep,
		SSLSelfSigned:  p.SSLSelfSigned,
		Packages:       p.Packages,
		Cron:           p.Cron,
		Fail2banIgnore: p.Fail2banIgnore,
	}
	if v.DeployUser == "" {
		v.DeployUser = env.SSH.User
	}
	if v.DocRoot == "" {
		v.DocRoot = DefaultDocRoot
	}
	if v.WebServer == "" && r != nil {
		switch {
		case r.Vhost != "":
			v.WebServer = r.Vhost
		case r.Get("nginx").Status != Absent:
			v.WebServer = "nginx"
		case r.Get("apache").Status != Absent:
			v.WebServer = "apache"
		}
	}
	if v.WebServer == "" {
		v.WebServer = "nginx"
	}
	if v.Database == "" && r != nil {
		switch {
		case r.Get("postgres").Status != Absent:
			v.Database = "postgres"
		case r.Get("mariadb").Status != Absent:
			v.Database = "mariadb"
		}
	}
	safe := regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(project, "_")
	if v.DBName == "" {
		v.DBName = safe
	}
	if v.DBUser == "" {
		v.DBUser = safe + "_user"
	}

	b := env.Builds
	for _, be := range backends {
		switch be.Kind {
		case "php":
			if v.PHPVersion == "" && r != nil {
				v.PHPVersion = r.PHPVersion
			}
			if v.PHPVersion == "" {
				v.PHPVersion = Versions("php")[0]
			}
		case "python":
			if v.ProxyPort == 0 && b.Python.WebPort > 0 {
				v.ProxyPort = b.Python.WebPort
			}
			// python_command: python3.13 means the app runs on that version
			if v.PythonVersion == "" {
				v.PythonVersion = strings.TrimPrefix(b.Python.PythonCommand, "python")
				if !strings.HasPrefix(v.PythonVersion, "3.") {
					v.PythonVersion = ""
				}
			}
		case "go":
			if v.ProxyPort == 0 {
				v.ProxyPort = 8080
			}
		case "node":
			if v.NodeMajor == "" {
				v.NodeMajor = majorRe.FindString(be.Detail)
			}
			if v.NodeMajor == "" {
				v.NodeMajor = Versions("node")[0]
			}
			if v.ProxyPort == 0 {
				v.ProxyPort = 3000
			}
		}
		if v.Backend == "" {
			v.Backend = be.Kind
		}
	}
	if len(env.Services) > 0 {
		v.AppService = env.Services[0].Name
	} else if v.Backend != "" && (v.Backend != "php" || v.FrankenPHP) {
		v.AppService = safe
	}
	// A PHP app served by FPM takes precedence: proxying only applies without PHP.
	// FrankenPHP is a service: nginx proxies index.php to it.
	if hasBackend(backends, "php") {
		switch {
		case !v.FrankenPHP:
			v.ProxyPort = 0
		case p.ProxyPort == 0:
			v.ProxyPort = 8000
		}
	}
	return v
}

// Preselect returns the services to tick by default: what the backends need and the
// provision block asks for, minus what's already active.
func Preselect(backends []Backend, v Vars, env *config.Environment, r *Report) []string {
	want := []string{"base", v.WebServer, "deploy_user", "sudoers", "dirs"}
	for _, be := range backends {
		switch be.Kind {
		case "php":
			if v.FrankenPHP {
				want = append(want, "frankenphp")
			} else {
				want = append(want, "php")
			}
		case "python":
			want = append(want, "python")
		case "go":
		case "node":
			want = append(want, "node")
		}
	}
	if v.Database == "postgres" || v.Database == "mariadb" {
		want = append(want, v.Database)
	}
	if env.Provision.Redis {
		want = append(want, "redis")
	}
	if env.Provision.Dragonfly {
		want = append(want, "dragonfly")
	}
	if env.Provision.SSL {
		want = append(want, "ssl")
	}
	if env.Provision.UFW {
		want = append(want, "ufw")
	}
	if env.Provision.Fail2ban {
		want = append(want, "fail2ban")
	}
	if env.Provision.Backup {
		want = append(want, "backup")
	}
	if len(env.Provision.Cron) > 0 {
		want = append(want, "cron")
	}
	var out []string
	for _, id := range want {
		if id == "" || (r != nil && r.Get(id).Status != Absent) || Conflicts(r, []string{id}, v) != nil {
			continue
		}
		out = append(out, id)
	}
	return ordered(out)
}

func hasBackend(bs []Backend, kind string) bool {
	for _, b := range bs {
		if b.Kind == kind {
			return true
		}
	}
	return false
}

// ProposedServices returns the deploy.yml `services` the app needs: the ones already
// defined, else one for a Python/Go/Node app (Go and Python derive exec from their
// build; Node runs `npm start`). The web server proxies to PORT.
func ProposedServices(env *config.Environment, v Vars) []config.ServiceConfig {
	if v.FrankenPHP && len(env.Services) == 0 && v.AppService != "" {
		return []config.ServiceConfig{{Name: v.AppService, Exec: FrankenPHPExec(v.Project), Reload: FrankenPHPReload(v.RemotePath)}}
	}
	if len(env.Services) > 0 || v.Backend == "" || v.Backend == "php" || v.AppService == "" {
		return env.Services
	}
	s := config.ServiceConfig{Name: v.AppService}
	if v.ProxyPort > 0 {
		s.Environment = map[string]string{"PORT": strconv.Itoa(v.ProxyPort)}
	}
	if v.Backend == "node" {
		s.Exec = "npm start"
	}
	return []config.ServiceConfig{s}
}
