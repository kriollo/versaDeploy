package provision

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	versassh "github.com/user/versaDeploy/internal/ssh"
)

//go:embed scripts/*.sh templates/*.tmpl
var assets embed.FS

var templates = template.Must(template.ParseFS(assets, "templates/*.tmpl"))

// Vars are the parameters of an install run. They reach the scripts as shell variables.
type Vars struct {
	Project    string
	Domain     string
	WebServer  string // apache | nginx
	PHPVersion string
	RemotePath string
	DocRoot    string // relative to RemotePath, default DefaultDocRoot
	DeployUser string
	Database   string // postgres | mariadb | none
	DBName     string
	DBUser     string
	DBPassword string
	AdminEmail string
	SSHPort    int    // port of the admin connection, kept open by UFW
	PubKey     string // added to the deploy user's authorized_keys
	Backend    string // php | python | go | node (first detected)
	ProxyPort  int    // reverse proxy target; 0 = PHP-FPM
	AppService string // name of the app's service (deploy.yml services), for detection and logs
	// ServicesSudoers is the script printing the sudoers lines `services` need
	// (deployer.SudoersScript); the sudoers step installs them.
	ServicesSudoers string
	// ServicesInstall is the script installing their init files as root
	// (deployer.ServiceInstallScript): deploys only restart them.
	ServicesInstall string
	NodeMajor       string
	// PythonVersion is installed next to the system Python ("" = system python3)
	PythonVersion string
	Redis         bool
	Aliases       []string // extra server names (www.example.com, other.com)
	// ResetDBPassword sets DBPassword on an existing DB user; by default an existing
	// user keeps its password, so other apps using it don't lose access.
	ResetDBPassword bool
	BackupKeep      int // daily DB backups kept (default 7)
}

// DefaultDocRoot is where releases keep the app code (versaDeploy uploads it under app/).
const DefaultDocRoot = "current/app"

// DocRootPath is the absolute document root.
func (v Vars) DocRootPath() string {
	d := strings.Trim(v.DocRoot, "/")
	if d == "" {
		d = DefaultDocRoot
	}
	return path.Join(v.RemotePath, d)
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+$`)

// Values the server reports end up in root scripts: accept only what apt and
// systemd can actually print.
var (
	phpVersionRe = regexp.MustCompile(`^\d+\.\d+$`)
	debPkgRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)
)

// pathRe: remote_path and doc_root end up in vhosts and root scripts
var pathRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// systemDirs are never a remote_path: the dirs step chowns it to the deploy user.
var systemDirs = map[string]bool{"bin": true, "boot": true, "dev": true, "etc": true, "lib": true,
	"lib64": true, "proc": true, "root": true, "run": true, "sbin": true, "sys": true, "usr": true}

func checkRemotePath(p string) error {
	c := path.Clean(p)
	parts := strings.Split(strings.Trim(c, "/"), "/")
	switch {
	case !strings.HasPrefix(p, "/") || !pathRe.MatchString(p):
		return fmt.Errorf("remote_path must be an absolute path of letters, digits, . _ - /")
	case len(parts) < 2 || systemDirs[parts[0]] || c == "/var/lib" || c == "/var/log":
		return fmt.Errorf("remote_path %q is a system directory: use one for the project, e.g. /var/www/app", p)
	}
	return nil
}

// Validate checks the vars the selected steps need. Names end up in config files and
// SQL, so they are restricted to safe characters.
func (v Vars) Validate(steps []string) error {
	need := func(cond bool, msg string) error {
		if !cond {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
	for _, s := range steps {
		var err error
		switch s {
		case "php":
			err = need(v.PHPVersion != "", "php_version is required (e.g. 8.4)")
		case "apache", "nginx", "ssl":
			if err = need(nameRe.MatchString(v.Domain), "domain is required (e.g. app.example.com)"); err == nil && s != "ssl" && v.ProxyPort == 0 {
				err = need(v.PHPVersion != "", "php_version is required for a PHP-FPM vhost (or set proxy_port)")
			}
		case "postgres", "mariadb":
			// the password is checked on the server: an existing user doesn't need one
			if err = need(identRe.MatchString(v.DBName) && identRe.MatchString(v.DBUser), "db_name and db_user must be letters, digits or _"); err == nil && v.ResetDBPassword {
				err = need(v.DBPassword != "", "db_password is required to reset the password")
			}
		case "deploy_user":
			if err = need(nameRe.MatchString(v.DeployUser), "deploy_user is required"); err == nil {
				err = need(strings.HasPrefix(strings.TrimSpace(v.PubKey), "ssh-") || strings.HasPrefix(strings.TrimSpace(v.PubKey), "ecdsa-"),
					"no public key: the deploy user is created with <ssh.key_path>.pub from this machine (create it with ssh-keygen -y -f KEY > KEY.pub)")
			}
		case "sudoers":
			err = need(nameRe.MatchString(v.DeployUser), "deploy_user is required")
		case "dirs":
			err = need(v.RemotePath != "", "remote_path is required")
		case "backup":
			if err = need(v.Database == "postgres" || v.Database == "mariadb", "select PostgreSQL or MariaDB for backups"); err == nil {
				err = need(identRe.MatchString(v.DBName) && v.RemotePath != "" && nameRe.MatchString(v.DeployUser), "db_name, remote_path and deploy_user are required")
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	// checked for every step: they also reach sudoers, unit names and vhosts
	format := []struct {
		val string
		re  string
		msg string
	}{
		{v.PHPVersion, `^\d\.\d$`, "php_version must look like 8.4"},
		{v.PythonVersion, `^3\.\d+$`, "python_version must look like 3.13"},
		{v.NodeMajor, `^\d+$`, "node_major must be a number like 24"},
		{v.Project, nameRe.String(), "project must be letters, digits, . _ -"},
		{v.WebServer, `^(nginx|apache)$`, "web_server must be nginx or apache"},
		{v.AppService, nameRe.String(), "invalid service name"},
	}
	for _, f := range format {
		if f.val != "" && !regexp.MustCompile(f.re).MatchString(f.val) {
			return fmt.Errorf("%s (got %q)", f.msg, f.val)
		}
	}
	if v.RemotePath != "" {
		if err := checkRemotePath(v.RemotePath); err != nil {
			return err
		}
	}
	if d := v.DocRoot; d != "" && (!pathRe.MatchString(d) || strings.Contains("/"+d+"/", "/../")) {
		return fmt.Errorf("doc_root must be a relative path inside remote_path (got %q)", d)
	}
	if strings.ContainsAny(strings.TrimSpace(v.PubKey), "\r\n") {
		return fmt.Errorf("the public key file must hold a single key")
	}
	for _, a := range v.Aliases {
		if !nameRe.MatchString(a) {
			return fmt.Errorf("invalid alias %q", a)
		}
	}
	// vhosts are rendered whenever a web server is set, whatever the steps
	if v.Domain != "" && !nameRe.MatchString(v.Domain) {
		return fmt.Errorf("invalid domain %q", v.Domain)
	}
	if v.AdminEmail != "" && !emailRe.MatchString(v.AdminEmail) {
		return fmt.Errorf("invalid admin_email %q", v.AdminEmail)
	}
	// sudoers reads ALL (and other reserved words) as "every user"
	if u := strings.ToUpper(v.DeployUser); u == "ALL" || u == "ROOT" && v.DeployUser != "root" {
		return fmt.Errorf("deploy_user %q is reserved", v.DeployUser)
	}
	if s := v.SSHPort; s == 0 {
		return fmt.Errorf("ssh port unknown")
	}
	return nil
}

// Bundle is what an install run uploads: file name → content. run.sh is the entry point.
type Bundle map[string][]byte

// Build assembles the install bundle for the selected steps, run in catalog order.
func Build(selected []string, v Vars) (Bundle, error) {
	steps := ordered(selected)
	if len(steps) == 0 {
		return nil, fmt.Errorf("nothing selected")
	}
	if v.AdminEmail == "" && v.Domain != "" {
		v.AdminEmail = "admin@" + v.Domain
	}
	if err := v.Validate(steps); err != nil {
		return nil, err
	}
	b := Bundle{"env": []byte(v.env())}

	var script bytes.Buffer
	common, _ := assets.ReadFile("scripts/common.sh")
	script.Write(common)
	for _, id := range steps {
		if id == "app_service" {
			continue // installed and restarted by every deploy (deploy.yml services)
		}
		body, err := assets.ReadFile("scripts/" + id + ".sh")
		if err != nil {
			return nil, fmt.Errorf("no script for %s", id)
		}
		svc, _ := Lookup(id)
		// each step is a function, so a step can end early with `return`
		fmt.Fprintf(&script, "\nsection %s\nstep_%s() {\n", versassh.ShellQuote(svc.Label), id)
		script.Write(body)
		fmt.Fprintf(&script, "\n}\nstep_%s\n", id)
	}
	script.WriteString("\nsection 'Done'\nok 'Provisioning finished'\n")
	b["run.sh"] = script.Bytes()

	if v.WebServer != "" {
		for _, ssl := range []bool{false, true} {
			out, err := RenderVhost(v, ssl)
			if err != nil {
				return nil, err
			}
			name := "vhost.conf"
			if ssl {
				name = "vhost-ssl.conf"
			}
			b[name] = []byte(out)
		}
	}
	if v.ServicesSudoers != "" {
		b["services-sudoers.sh"] = []byte(v.ServicesSudoers)
	}
	if v.ServicesInstall != "" {
		b["services-install.sh"] = []byte(v.ServicesInstall)
	}
	return b, nil
}

// RenderVhost renders the web server config, with or without the SSL server blocks.
func RenderVhost(v Vars, ssl bool) (string, error) {
	data := struct {
		Vars
		SSL         bool
		DocRootPath string
	}{v, ssl, v.DocRootPath()}
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, v.WebServer+".conf.tmpl", data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// UpgradeBundle upgrades the given packages and reloads units. With refreshOnly it only
// refreshes the apt index (apt-get update).
func UpgradeBundle(pkgs, units []string, refreshOnly bool) Bundle {
	var s strings.Builder
	common, _ := assets.ReadFile("scripts/common.sh")
	s.Write(common)
	s.WriteString("ok 'Package index refreshed'\n") // common.sh runs apt-get update
	if !refreshOnly {
		var q []string
		for _, p := range pkgs {
			if debPkgRe.MatchString(p) {
				q = append(q, versassh.ShellQuote(p))
			}
		}
		fmt.Fprintf(&s, "\nsection 'Upgrade'\napt-get install --only-upgrade -y -o Dpkg::Options::=--force-confold %s\n", strings.Join(q, " "))
		for _, u := range units {
			if !nameRe.MatchString(u) {
				continue
			}
			fmt.Fprintf(&s, "systemctl try-reload-or-restart %s 2>/dev/null && ok %s || true\n", versassh.ShellQuote(u), versassh.ShellQuote("Reloaded "+u))
		}
		s.WriteString("[ -f /var/run/reboot-required ] && warn 'A reboot is required to finish the upgrade'\nok 'Upgrade finished'\n")
	}
	return Bundle{"env": nil, "run.sh": []byte(s.String())}
}

// Exec runs one command as root over the admin connection, streaming its output.
func Exec(client *versassh.Client, user, cmd string, w io.Writer) error {
	if user != "root" {
		cmd = "sudo -n /bin/sh -c " + versassh.ShellQuote(cmd)
	}
	// stdout and stderr are copied by separate goroutines: serialize them, or a plain
	// bytes.Buffer loses output
	sw := &syncWriter{w: w}
	return client.ExecuteCommandStreaming("/bin/sh -c "+versassh.ShellQuote(cmd), sw, sw)
}

// Run uploads the bundle to a private temp dir and runs run.sh as root, streaming the
// output to w. Secrets travel in the env file, never on a command line.
func Run(client *versassh.Client, user string, b Bundle, w io.Writer) error {
	dir := fmt.Sprintf("/tmp/versa-provision-%d", time.Now().UnixNano())
	if _, err := client.ExecuteCommand("umask 077 && mkdir " + dir); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, data := range b {
		// 0600: env holds the DB password (the dir is 0700 too)
		if err := client.WriteRemoteFileMode(dir+"/"+name, data, 0600); err != nil {
			client.ExecuteCommand("rm -rf " + dir)
			return fmt.Errorf("upload %s: %w", name, err)
		}
	}
	// The trap also removes the bundle (and its secrets) when the SSH session drops.
	return Exec(client, user, fmt.Sprintf("trap 'rm -rf %[1]s' EXIT HUP INT TERM; bash %[1]s/run.sh", dir), w)
}

// SuggestedReload returns the services_reload commands matching v.
func SuggestedReload(v Vars) []string {
	var out []string
	if v.PHPVersion != "" && v.ProxyPort == 0 && v.WebServer != "" {
		out = append(out, "sudo systemctl reload php"+v.PHPVersion+"-fpm")
	}
	switch v.WebServer {
	case "nginx":
		out = append(out, "sudo systemctl reload nginx")
	case "apache":
		out = append(out, "sudo systemctl reload apache2")
	}
	return out
}

func (v Vars) env() string {
	q := versassh.ShellQuote
	kv := [][2]string{
		{"PROJECT_NAME", v.Project}, {"DOMAIN", v.Domain}, {"WEB_SERVER", v.WebServer},
		{"PHP_VERSION", v.PHPVersion}, {"REMOTE_PATH", path.Clean(v.RemotePath)}, {"DOC_ROOT", v.DocRootPath()},
		{"DEPLOY_USER", v.DeployUser}, {"DATABASE", v.Database}, {"DB_NAME", v.DBName},
		{"DB_USER", v.DBUser}, {"DB_PASS", v.DBPassword}, {"ADMIN_EMAIL", v.AdminEmail},
		{"SSH_PORT", strconv.Itoa(v.SSHPort)}, {"PUBKEY", strings.TrimSpace(v.PubKey)},
		{"PROXY_PORT", strconv.Itoa(v.ProxyPort)},
		{"NODE_MAJOR", v.NodeMajor}, {"INSTALL_REDIS", strconv.FormatBool(v.Redis)},
		{"ALIASES", strings.Join(v.Aliases, " ")}, {"BACKUP_KEEP", strconv.Itoa(v.keep())},
		{"RESET_DB_PASSWORD", strconv.FormatBool(v.ResetDBPassword)},
		{"PYTHON_VERSION", v.PythonVersion},
	}
	var s strings.Builder
	for _, p := range kv {
		fmt.Fprintf(&s, "%s=%s\n", p[0], q(p[1]))
	}
	return s.String()
}

func ordered(selected []string) []string {
	var out []string
	for _, s := range Catalog {
		for _, id := range selected {
			if id == s.ID {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func (v Vars) keep() int {
	if v.BackupKeep > 0 {
		return v.BackupKeep
	}
	return 7
}

// BackupUnit is the systemd unit that dumps the project's database.
func BackupUnit(project string) string { return "versa-backup-" + project }

// BackupNowCommand runs the backup script installed by the backup step; it prints its
// own result (reading the journal right after `systemctl start` races).
func BackupNowCommand(project string) string {
	return versassh.ShellQuote("/usr/local/sbin/" + BackupUnit(project))
}

// LogsCommand prints the recent logs of a service: its systemd journal and log files.
// ponytail: snapshot of the last lines, no follow mode; add one when the SSH client can
// cancel a streaming command.
func LogsCommand(id string, v Vars, phpVersion string) string {
	svc, _ := Lookup(id)
	units := svc.UnitNames(phpVersion)
	var files []string
	switch id {
	case "nginx":
		files = []string{"/var/log/nginx/error.log", "/var/log/nginx/access.log"}
	case "apache":
		files = []string{"/var/log/apache2/" + v.Domain + "-error.log", "/var/log/apache2/error.log"}
	case "php":
		files = []string{"/var/log/php" + phpVersion + "-fpm.log"}
	case "redis":
		files = []string{"/var/log/redis/redis-server.log"}
	case "app_service":
		units = []string{v.AppService}
	case "backup":
		units = []string{BackupUnit(v.Project) + ".service"}
	case "ssl":
		files = []string{"/var/log/letsencrypt/letsencrypt.log"}
	case "ufw":
		files = []string{"/var/log/ufw.log"}
	}
	q := versassh.ShellQuote
	var s strings.Builder
	for _, u := range units {
		if u == "" {
			continue
		}
		fmt.Fprintf(&s, "echo; echo %s; journalctl -u %s -n 100 --no-pager -o short-iso 2>&1 | tail -n 100\n", q("══════ journalctl -u "+u+" ══════"), q(u))
	}
	for _, f := range files {
		fmt.Fprintf(&s, "if [ -f %s ]; then echo; echo %s; tail -n 100 %s; fi\n", q(f), q("══════ "+f+" ══════"), q(f))
	}
	if s.Len() == 0 {
		return "echo 'No logs for this service'"
	}
	return s.String()
}

// Conflicts reports selected services that would add an alternative to one already on
// the server: a second web server or a second PHP-FPM version. MySQL vs MariaDB is
// handled by the MariaDB step, which reuses an existing MySQL.
func Conflicts(r *Report, selected []string, v Vars) error {
	if r == nil {
		return nil
	}
	has := func(id string) bool {
		for _, s := range selected {
			if s == id {
				return true
			}
		}
		return false
	}
	switch {
	case has("nginx") && r.Get("apache").Status != Absent:
		return fmt.Errorf("Apache is already installed on this server: use it instead of nginx")
	case has("apache") && r.Get("nginx").Status != Absent:
		return fmt.Errorf("nginx is already installed on this server: use it instead of Apache")
	case has("php") && r.PHPVersion != "" && v.PHPVersion != r.PHPVersion:
		return fmt.Errorf("PHP-FPM %s is already installed: set PHP version to %s instead of %s", r.PHPVersion, r.PHPVersion, v.PHPVersion)
	}
	if nv := r.Get("node").Version; has("node") && nv != "" && v.NodeMajor != "" && majorRe.FindString(nv) != v.NodeMajor {
		return fmt.Errorf("Node.js %s is already installed: set Node major to %s instead of %s", nv, majorRe.FindString(nv), v.NodeMajor)
	}
	return nil
}

// syncWriter serializes writes coming from the stdout and stderr goroutines.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
