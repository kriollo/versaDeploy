package provision

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/user/versaDeploy/internal/config"
)

func TestParseReport(t *testing.T) {
	out := strings.Join([]string{
		"os_id=ubuntu",
		"apt=1",
		"svc_base=installed",
		"svc_nginx=active 1.24.0 (Ubuntu)",
		"php_fpm=8.3",
		"svc_php=installed 8.3.6",
		"svc_python=absent 3.12.3 (no venv)",
		"vhost=nginx",
		"upg nginx/noble-updates,noble-security 1.24.0-2ubuntu7.1 amd64 [upgradable from: 1.24.0-2ubuntu7]",
		"upg php8.3-fpm/noble-updates 8.3.6-0ubuntu0.24.04.3 amd64 [upgradable from: 8.3.6-0ubuntu0.24.04.2]",
		"upg libc6/noble-security 2.39-0ubuntu8.4 amd64 [upgradable from: 2.39-0ubuntu8.3]",
		"reboot=1",
		"reboot_pkgs=linux-image-6.8.0-50-generic libc6 ",
		"unattended=inactive",
		"apt_age=90000",
		"garbage line",
	}, "\r\n")
	r := ParseReport(out)
	if !r.Apt || r.OSID != "ubuntu" || r.PHPVersion != "8.3" || r.Vhost != "nginx" || r.AptAge != 90000 {
		t.Fatalf("bad report: %+v", r)
	}
	if got := r.Get("nginx"); got.Status != Active || got.Version != "1.24.0 (Ubuntu)" {
		t.Errorf("nginx = %+v", got)
	}
	if got := r.Get("python"); got.Status != Absent || got.Version != "3.12.3 (no venv)" {
		t.Errorf("python = %+v", got)
	}
	if r.Get("redis").Status != Absent {
		t.Error("unreported service should be absent")
	}
	if !r.RebootRequired || len(r.RebootPkgs) != 2 || r.Unattended != "inactive" {
		t.Errorf("reboot/unattended: %+v", r)
	}
	if len(r.Upgrades) != 3 {
		t.Fatalf("upgrades = %+v", r.Upgrades)
	}
	if u := r.Upgrades[0]; u.Pkg != "nginx" || !u.Security || u.From != "1.24.0-2ubuntu7" || u.To != "1.24.0-2ubuntu7.1" {
		t.Errorf("upgrade[0] = %+v", u)
	}
	if n := len(r.UpgradesFor("nginx")); n != 1 {
		t.Errorf("nginx upgrades = %d", n)
	}
	if n := len(r.UpgradesFor("php")); n != 1 {
		t.Errorf("php upgrades = %d (php{v}-* with v=8.3)", n)
	}
	if n := len(r.SecurityUpgrades()); n != 2 {
		t.Errorf("security upgrades = %d", n)
	}
}

func TestDetectScriptRunsLocally(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	out, err := exec.Command(sh, "-c", DetectScript(DetectParams{Project: "x", RemotePath: t.TempDir(), DeployUser: "nobody"})).CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	r := ParseReport(string(out))
	if r.Get("dirs").Status != Installed {
		t.Errorf("dirs not detected:\n%s", out)
	}
}

func testVars() Vars {
	return Vars{
		Project: "versatodo", Domain: "todo.example.org", WebServer: "nginx", PHPVersion: "8.4",
		RemotePath: "/var/www/versatodo", DocRoot: "current", DeployUser: "versadeploy",
		Database: "postgres", DBName: "todo", DBUser: "todo_user", DBPassword: "s3'cret",
		SSHPort: 22, Backend: "php", PubKey: "ssh-ed25519 AAAAtest e2e",
		Cron: []string{"* * * * * php versaCLI app:schedule:run >> versaWYS/logs/cron.log 2>&1"},
	}
}

func TestCron(t *testing.T) {
	for l, ok := range map[string]bool{
		"*/5 * * * * php versaCLI app:schedule:run": true,
		"@daily  php versaCLI backup":               true,
		"0 3 * * 1-5 date +\\%F >> x.log":           true,  // escaped %
		"0 3 * * * date +%F":                        false, // cron would cut the command there
		"@sometimes php x":                          false,
		"* * * * php x":                             false, // 4 fields: "php" is no weekday
		"0 9 * jan mon-fri php x":                   true,
		"* * * *":                                   false,
		"* * * * * ; rm -rf /\n* * * * * id":        false,
	} {
		if _, _, err := cronLine(l); (err == nil) != ok {
			t.Errorf("cronLine(%q) err = %v, want ok=%v", l, err, ok)
		}
	}
	v := testVars()
	v.DocRoot = "current/app/public" // jobs still run in the app dir
	got := v.cronTab()
	want := "* * * * * versadeploy cd '/var/www/versatodo/current/app' && umask 002 && php versaCLI app:schedule:run >> versaWYS/logs/cron.log 2>&1\n"
	if !strings.HasSuffix(got, want) || !strings.Contains(got, "PATH=") {
		t.Errorf("cronTab:\n%s\nwant suffix:\n%s", got, want)
	}
	if CronFile("app.example") != "/etc/cron.d/versa-app_example" {
		t.Error(CronFile("app.example"))
	}
}

func TestRenderVhost(t *testing.T) {
	for _, ws := range []string{"nginx", "apache"} {
		for _, ssl := range []bool{false, true} {
			v := testVars()
			v.WebServer = ws
			v.AdminEmail = "a@b.c"
			out, err := RenderVhost(v, ssl)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"todo.example.org", "/var/www/versatodo/current", "php8.4-fpm.sock"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s ssl=%v: missing %q", ws, ssl, want)
				}
			}
			for _, bad := range []string{"example.com", "mi-proyecto", "<no value>"} {
				if strings.Contains(out, bad) {
					t.Errorf("%s ssl=%v: contains %q", ws, ssl, bad)
				}
			}
			if hasCert := strings.Contains(out, "/etc/letsencrypt/live/todo.example.org/fullchain.pem"); hasCert != ssl {
				t.Errorf("%s ssl=%v: certificate lines present=%v", ws, ssl, hasCert)
			}

			v.ProxyPort = 8000
			v.Backend = "python"
			out, _ = RenderVhost(v, ssl)
			if !strings.Contains(out, "127.0.0.1:8000") || strings.Contains(out, "fpm.sock") {
				t.Errorf("%s proxy ssl=%v: want proxy to :8000 and no FPM", ws, ssl)
			}
		}
	}
}

func TestBuildBundle(t *testing.T) {
	v := testVars()
	v.ServicesSudoers = "echo 'versadeploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart todo-api'"
	all := make([]string, 0, len(Catalog))
	for _, s := range Catalog {
		if s.ID != "frankenphp" { // the alternative to php (TestFrankenPHP)
			all = append(all, s.ID)
		}
	}
	b, err := Build(all, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"run.sh", "env", "vhost.conf", "vhost-ssl.conf", "services-sudoers.sh"} {
		if len(b[f]) == 0 {
			t.Errorf("bundle misses %s", f)
		}
	}
	if strings.Contains(string(b["run.sh"]), "s3'cret") {
		t.Error("secret leaked into run.sh")
	}
	if bash, err := exec.LookPath("bash"); err == nil {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "run.sh"), b["run.sh"], 0600)
		if out, err := exec.Command(bash, "-n", filepath.Join(dir, "run.sh")).CombinedOutput(); err != nil {
			t.Errorf("bash -n: %v\n%s", err, out)
		}
		os.WriteFile(filepath.Join(dir, "env"), b["env"], 0600)
		out, err := exec.Command(bash, "-c", `. "$1/env" && printf %s "$DB_PASS"`, "_", dir).Output()
		if err != nil || string(out) != "s3'cret" {
			t.Errorf("DB_PASS from env = %q, %v", out, err)
		}
	}

	if _, err := Build([]string{"postgres"}, Vars{SSHPort: 22, DBName: "x;drop", DBUser: "u", DBPassword: "p"}); err == nil {
		t.Error("unsafe db_name accepted")
	}
	if _, err := Build([]string{"nginx"}, Vars{SSHPort: 22, WebServer: "nginx"}); err == nil {
		t.Error("missing domain accepted")
	}
}

func TestSuggestedReload(t *testing.T) {
	v := testVars()
	got := SuggestedReload(v)
	want := []string{"sudo systemctl reload php8.4-fpm", "sudo systemctl reload nginx"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestDetectBackends(t *testing.T) {
	write := func(dir, name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	env := &config.Environment{}

	php := t.TempDir()
	write(php, "composer.json", `{"require":{"php":">=8.3"}}`)
	write(php, "package.json", `{"scripts":{"start":"vite"}}`)
	if got := DetectBackends(env, php); len(got) != 1 || got[0].Kind != "php" || got[0].Detail != "php >=8.3" {
		t.Errorf("php repo: %+v", got)
	}

	node := t.TempDir()
	write(node, "package.json", `{"scripts":{"start":"node server.js"},"engines":{"node":">=20"}}`)
	got := DetectBackends(env, node)
	if len(got) != 1 || got[0].Kind != "node" {
		t.Fatalf("node repo: %+v", got)
	}
	v := Defaults(env, "api", got, nil)
	if v.NodeMajor != "20" || v.ProxyPort != 3000 || v.AppService != "api" {
		t.Errorf("node defaults: %+v", v)
	}

	static := t.TempDir()
	write(static, "package.json", `{"scripts":{"build":"vite build"}}`)
	if got := DetectBackends(env, static); len(got) != 0 {
		t.Errorf("static frontend detected as backend: %+v", got)
	}

	multi := t.TempDir()
	write(multi, "go.mod", "module x")
	write(multi, "pyproject.toml", "")
	if got := DetectBackends(env, multi); len(got) != 2 {
		t.Errorf("go+python repo: %+v", got)
	}

	// deploy.yml builds win over files
	py := &config.Environment{RemotePath: "/srv/app"}
	py.Builds.Python = config.PythonBuildConfig{Enabled: true, WebServer: true, WebPort: 8000, WebFramework: "fastapi"}
	py.Services = []config.ServiceConfig{{Name: "app-api"}}
	got = DetectBackends(py, php)
	if len(got) != 1 || got[0].Kind != "python" || got[0].Detail != "fastapi :8000" {
		t.Fatalf("builds: %+v", got)
	}
	v = Defaults(py, "app", got, nil)
	if v.ProxyPort != 8000 || v.AppService != "app-api" {
		t.Errorf("python defaults: %+v", v)
	}
	// nginx is active: its step only comes back for this project's missing vhost
	sel := Preselect(got, v, py, &Report{Services: map[string]State{"nginx": {Status: Active}}, Vhost: "nginx"})
	if !slices.Contains(sel, "python") || slices.Contains(sel, "app_service") || slices.Contains(sel, "nginx") {
		t.Errorf("preselect: %v", sel)
	}
	if sel := Preselect(got, v, py, &Report{Services: map[string]State{"nginx": {Status: Active}}}); !slices.Contains(sel, "nginx") {
		t.Errorf("a second project on an nginx server needs its vhost: %v", sel)
	}
	// defined services are kept as they are; none defined → one proposed for the app
	if got := ProposedServices(py, v); len(got) != 1 || got[0].Name != "app-api" || got[0].Environment != nil {
		t.Errorf("existing services changed: %+v", got)
	}
	nodeEnv := &config.Environment{}
	got2 := ProposedServices(nodeEnv, Vars{Backend: "node", AppService: "api", ProxyPort: 3000})
	if len(got2) != 1 || got2[0].Exec != "npm start" || got2[0].Environment["PORT"] != "3000" {
		t.Errorf("node proposal: %+v", got2)
	}
	if got := ProposedServices(nodeEnv, Vars{Backend: "php", AppService: "x"}); len(got) != 0 {
		t.Errorf("php app needs no service: %+v", got)
	}
}

func TestAliasesSitesBackupsLogs(t *testing.T) {
	v := testVars()
	v.Aliases = []string{"www.todo.example.org", "todo.example.net"}
	for _, ws := range []string{"nginx", "apache"} {
		v.WebServer = ws
		out, err := RenderVhost(v, true)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "www.todo.example.org todo.example.net") {
			t.Errorf("%s: aliases missing", ws)
		}
	}
	if _, err := Build([]string{"nginx"}, Vars{SSHPort: 22, WebServer: "nginx", Domain: "a.b", PHPVersion: "8.4", Aliases: []string{"bad;name"}}); err == nil {
		t.Error("unsafe alias accepted")
	}
	if _, err := Build([]string{"backup"}, Vars{SSHPort: 22, Database: "none"}); err == nil {
		t.Error("backup without database accepted")
	}

	r := ParseReport("site=versatodo|todo.example.org www.todo.example.org|/var/www/t/current/app\nsite=other.conf|other.com|\nbackup_last=todo-20261105-030000.sql.gz 4.0K\nsvc_backup=active")
	if len(r.Sites) != 2 || r.Sites[0].Names != "todo.example.org www.todo.example.org" || r.Sites[1].Root != "" {
		t.Errorf("sites = %+v", r.Sites)
	}
	if r.BackupLast != "todo-20261105-030000.sql.gz 4.0K" || r.Get("backup").Status != Active {
		t.Errorf("backup = %q %+v", r.BackupLast, r.Get("backup"))
	}

	for _, id := range []string{"nginx", "php", "app_service", "backup"} {
		cmd := LogsCommand(id, Vars{Project: "p", AppService: "p-api", Domain: "d.com"}, "8.4")
		if bash, err := exec.LookPath("bash"); err == nil {
			if out, err := exec.Command(bash, "-n", "-c", cmd).CombinedOutput(); err != nil {
				t.Errorf("%s logs cmd: %v %s", id, err, out)
			}
		}
	}
	if !strings.Contains(LogsCommand("php", Vars{}, "8.4"), "php8.4-fpm") {
		t.Error("php logs should read the php8.4-fpm journal")
	}
}

func TestMatchProjectSite(t *testing.T) {
	r := ParseReport("svc_apache=active 2.4.52\nsite=appcoros.conf|app.coros.cl|/var/www/versa/appcoros\nsite=appvserv-le-ssl.conf|app.vserv.cl|/var/www/versa/appvserv/current/app")
	r.matchProjectSite("/var/www/versa/appvserv")
	if r.Vhost != "apache" || r.ProjectSite != "appvserv-le-ssl.conf" {
		t.Errorf("vhost=%q site=%q", r.Vhost, r.ProjectSite)
	}
	r = ParseReport("site=appvserv2.conf|x|/var/www/versa/appvserv2/current")
	if r.matchProjectSite("/var/www/versa/appvserv"); r.ProjectSite != "" {
		t.Error("prefix match must stop at a path boundary")
	}
}

func TestConflicts(t *testing.T) {
	r := ParseReport("svc_apache=active 2.4.52\nphp_fpm=8.4\nsvc_php=active 8.4.27")
	v := Vars{PHPVersion: "8.4"}
	if Conflicts(r, []string{"nginx"}, v) == nil {
		t.Error("nginx over Apache allowed")
	}
	if Conflicts(r, []string{"apache", "php"}, v) != nil {
		t.Error("re-running the installed web server/PHP must be allowed")
	}
	if err := Conflicts(r, []string{"php"}, Vars{PHPVersion: "8.3"}); err != nil {
		t.Errorf("another project's PHP-FPM version goes next to 8.4: %v", err)
	}
	env := &config.Environment{}
	env.Provision.WebServer = "nginx"
	if sel := Preselect([]Backend{{Kind: "php"}}, Vars{WebServer: "nginx", PHPVersion: "8.4"}, env, r); slices.Contains(sel, "nginx") {
		t.Errorf("preselected nginx on an Apache server: %v", sel)
	}
}

func TestDeployUserNeedsPubKeyAndPasswordRules(t *testing.T) {
	v := testVars()
	v.PubKey = ""
	if _, err := Build([]string{"deploy_user"}, v); err == nil || !strings.Contains(err.Error(), "public key") {
		t.Errorf("deploy_user without public key: %v", err)
	}
	v = testVars()
	v.DBPassword = ""
	if _, err := Build([]string{"postgres"}, v); err != nil {
		t.Errorf("an existing DB user needs no password, got %v", err)
	}
	v.ResetDBPassword = true
	if _, err := Build([]string{"postgres"}, v); err == nil {
		t.Error("reset without password accepted")
	}
	r := ParseReport("svc_node=installed v20.11.1")
	if Conflicts(r, []string{"node"}, Vars{NodeMajor: "22"}) != nil || Conflicts(r, []string{"node"}, Vars{NodeMajor: "20"}) != nil {
		t.Error("a project's Node major goes next to the server's (/opt/versa)")
	}
}

func TestSupportedVersions(t *testing.T) {
	var cycles []eolCycle
	raw := `[
	 {"cycle":"27","releaseDate":"2027-04-20","eol":"2030-04-30","lts":"2027-10-20"},
	 {"cycle":"26","releaseDate":"2026-04-22","eol":"2029-04-30","lts":"2026-10-28"},
	 {"cycle":"25","releaseDate":"2025-10-15","eol":"2026-06-01","lts":false},
	 {"cycle":"24","releaseDate":"2025-05-06","eol":"2028-04-30","lts":"2025-10-28"},
	 {"cycle":"22","releaseDate":"2024-04-24","eol":"2027-04-30","lts":"2024-10-29"},
	 {"cycle":"20","releaseDate":"2023-04-18","eol":"2026-04-30","lts":"2023-10-24"},
	 {"cycle":"x","releaseDate":"2020-01-01","eol":true,"lts":true}]`
	if err := json.Unmarshal([]byte(raw), &cycles); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if got := supportedCycles(cycles, true, now); !slices.Equal(got, []string{"26", "24", "22"}) {
		t.Errorf("node LTS = %v", got)
	}
	if got := supportedCycles(cycles, false, now); !slices.Equal(got, []string{"26", "24", "22"}) {
		t.Errorf("supported (25 and 20 are EOL, 27 unreleased) = %v", got)
	}
	for _, p := range []string{"php", "node", "python"} {
		if v := Versions(p); len(v) < 3 {
			t.Errorf("%s offers %v, want at least 3", p, v)
		}
	}
}

// Values that reach sudoers, vhosts and root scripts are refused when unsafe.
func TestValidateRejectsUnsafeValues(t *testing.T) {
	bad := map[string]func(*Vars){
		"remote_path /":        func(v *Vars) { v.RemotePath = "/" },
		"remote_path /etc/app": func(v *Vars) { v.RemotePath = "/etc/app" },
		"remote_path /var":     func(v *Vars) { v.RemotePath = "/var" },
		"remote_path newline":  func(v *Vars) { v.RemotePath = "/var/www/a\n}" },
		"doc_root escapes":     func(v *Vars) { v.DocRoot = "current/../../etc" },
		"php into sudoers":     func(v *Vars) { v.PHPVersion = "8.4, /bin/bash" },
		"project with slash":   func(v *Vars) { v.Project = "../x" },
		"two keys":             func(v *Vars) { v.PubKey = "ssh-ed25519 AAA a\nssh-rsa BBB b" },
		"unknown web server":   func(v *Vars) { v.WebServer = "caddy" },
	}
	for name, mut := range bad {
		v := testVars()
		mut(&v)
		if err := v.Validate([]string{"dirs", "sudoers"}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if v := testVars(); v.Validate([]string{"dirs", "sudoers"}) != nil {
		t.Errorf("valid vars refused: %v", v.Validate([]string{"dirs", "sudoers"}))
	}
}

func TestSelfSignedVhost(t *testing.T) {
	for _, ws := range []string{"nginx", "apache"} {
		v := testVars()
		v.WebServer, v.SSLSelfSigned = ws, true
		out, err := RenderVhost(v, true)
		if err != nil || !strings.Contains(out, "/etc/ssl/versa/todo.example.org/fullchain.pem") || strings.Contains(out, "/etc/letsencrypt/live") {
			t.Errorf("%s: self-signed vhost uses the wrong certificate (%v)", ws, err)
		}
	}
}

func TestConflictsCache(t *testing.T) {
	v := testVars()
	withRedis := ParseReport("svc_redis=active 7.0.15\n")
	withDfly := ParseReport("svc_dragonfly=active v2.1\n")
	empty := ParseReport("")
	for _, c := range []struct {
		r      *Report
		steps  []string
		refuse bool
	}{
		{empty, []string{"dragonfly"}, false},
		{empty, []string{"redis", "dragonfly"}, true},
		{withRedis, []string{"dragonfly"}, true},
		{withDfly, []string{"redis"}, true},
		{withDfly, []string{"dragonfly"}, false}, // re-run keeps it
	} {
		if err := Conflicts(c.r, c.steps, v); (err != nil) != c.refuse {
			t.Errorf("Conflicts(%v) = %v, want refuse=%v", c.steps, err, c.refuse)
		}
	}
}

// limit_req applies to the app only: static files must not use up the burst.
func TestNginxRateLimitOnlyDynamic(t *testing.T) {
	for _, proxy := range []int{0, 8000} {
		v := testVars()
		v.ProxyPort = proxy
		out, err := RenderVhost(v, false)
		if err != nil {
			t.Fatal(err)
		}
		var where []string
		block := "server"
		for _, l := range strings.Split(out, "\n") {
			if t := strings.TrimSpace(l); strings.HasPrefix(t, "location") {
				block = t
			}
			if strings.Contains(l, "limit_req zone=") {
				where = append(where, block)
			}
		}
		want := "location = /index.php {"
		if proxy != 0 {
			want = "location / {"
		}
		if len(where) != 1 || where[0] != want {
			t.Errorf("proxy %d: limit_req in %q, want only in %q", proxy, where, want)
		}
		// behind a proxy the app serves its own chunks: they get a proxy location with no limit
		if proxy != 0 && (strings.Count(out, "proxy_pass http://127.0.0.1:8000;") != 2 || !strings.Contains(out, `location ~* \.(js|mjs|css`)) {
			t.Errorf("proxy: static files must be proxied without limit_req:\n%s", out)
		}
	}
}

// FrankenPHP: nginx keeps the FPM vhost's rules and proxies only index.php, with the
// original URI; the Caddyfile listens on localhost and runs index.php for everything.
func TestFrankenPHP(t *testing.T) {
	v := testVars()
	v.FrankenPHP, v.ProxyPort, v.AppService, v.DocRoot = true, 8000, "versatodo", "current/app/public"
	b, err := Build([]string{"frankenphp", "nginx"}, v)
	if err != nil {
		t.Fatal(err)
	}
	vh := string(b["vhost-ssl.conf"])
	for _, want := range []string{"location = /index.php", "proxy_pass http://127.0.0.1:8000$request_uri;", "location ~ ^/(vendor|node_modules)/", "try_files $uri $uri/ /index.php?$query_string"} {
		if !strings.Contains(vh, want) {
			t.Errorf("vhost misses %q", want)
		}
	}
	if strings.Contains(vh, "fpm.sock") || strings.Count(vh, "proxy_pass") != 1 {
		t.Errorf("vhost: want one proxy_pass and no FPM:\n%s", vh)
	}
	cf := string(b["Caddyfile"])
	for _, want := range []string{"http://:8000 {", "bind 127.0.0.1", "root * /var/www/versatodo/current/app/public", "rewrite * /index.php", "admin off", "opcache.revalidate_path 1", "bind unix//var/www/versatodo/.frankenphp-opcache.sock|0600"} {
		if !strings.Contains(cf, want) {
			t.Errorf("Caddyfile misses %q:\n%s", want, cf)
		}
	}
	if _, err := Build([]string{"frankenphp"}, Vars{Project: "x", WebServer: "apache", SSHPort: 22, FrankenPHP: true, ProxyPort: 8000, AppService: "x"}); err == nil {
		t.Error("FrankenPHP behind Apache must be refused")
	}
	r := &Report{Services: map[string]State{"php": {Status: Active}}}
	if err := Conflicts(r, []string{"frankenphp"}, v); err != nil {
		t.Errorf("FrankenPHP for this app next to another app's PHP-FPM: %v", err)
	}
	env := &config.Environment{Provision: config.ProvisionConfig{FrankenPHP: true}}
	if s := ProposedServices(env, v); len(s) != 1 || s[0].Exec != "/usr/local/bin/frankenphp run --config /etc/frankenphp/versatodo.Caddyfile" || s[0].Reload != "curl -fsS --unix-socket /var/www/versatodo/.frankenphp-opcache.sock http://localhost/" {
		t.Errorf("proposed services: %+v", s)
	}
}

// The versa-scan filter bans scanners, never an app's normal traffic: versaSTORE's API
// errors and rate limits, payment webhooks, logins, its own 404s, and real paths that
// happen to look like scanner targets but exist (200/30x).
func TestFail2banScanFilter(t *testing.T) {
	script, _ := assets.ReadFile("scripts/fail2ban.sh")
	m := regexp.MustCompile(`(?m)^failregex = (.*)$`).FindSubmatch(script)
	if m == nil {
		t.Fatal("no failregex in fail2ban.sh")
	}
	re := regexp.MustCompile(strings.Replace(strings.TrimRight(string(m[1]), " \r"), "<HOST>", `(?P<host>\S+)`, 1) + " ")
	line := func(req string, status int) string {
		return fmt.Sprintf(`203.0.113.7 - - [09/Nov/2026:10:00:00 -0300] "%s HTTP/1.1" %d 162 "-" "Mozilla/5.0"`, req, status) + "\n"
	}
	legit := []string{
		line("GET /api/customers/42", 401), line("POST /api/orders/", 403), line("POST /api/coupons/validate", 422),
		line("GET /api/products/search?q=env", 429), line("GET /api/analytics", 429),
		line("POST /orders/webpay/confirm/3", 400), line("POST /orders/mercadopago/webhook/3", 404), line("POST /orders/flow/confirm/3", 403),
		line("POST /login/autentication", 401), line("POST /storefront/login", 401),
		line("GET /productos/zapatilla-env", 404), line("GET /build/assets/app-x.js", 404), line("GET /environment/settings", 404),
		line("GET /wp-login.php", 200), line("GET /phpmyadmin/", 302), line("GET /blog/wp-admin/", 301), // they exist here
		line("GET /admin/", 404), line("GET /administracion/sql-reports", 404), line("GET /docs/mysql-guide", 404),
	}
	for _, l := range legit {
		if re.MatchString(l) {
			t.Errorf("legit request would count as a scan: %s", l)
		}
	}
	scans := []string{
		line("GET /.env", 404), line("GET /api/.env.production", 404), line("GET /.git/config", 403), line("GET /wp-login.php", 404),
		line("POST /xmlrpc.php", 404), line("GET /phpMyAdmin/index.php", 404), line("GET /vendor/phpunit/phpunit/src/Util/PHP/eval-stdin.php", 404),
		line("GET /cgi-bin/luci", 404), line("GET /backup.sql", 404), line("GET /db.sql.gz", 444), line("GET /.aws/credentials", 404),
	}
	for _, l := range scans {
		if sm := re.FindStringSubmatch(l); sm == nil || sm[1] != "203.0.113.7" {
			t.Errorf("scan not caught: %s", l)
		}
	}
	if _, err := Build([]string{"fail2ban"}, Vars{Project: "x", SSHPort: 22, Fail2banIgnore: []string{"10.0.0.0/8", "nope"}}); err == nil {
		t.Error("an invalid fail2ban_ignore entry must be refused")
	}
}

// Nuxt has no "start" script: its service (node .output/server/index.mjs) or
// provision.node_major still make it a Node backend, so Node.js is preselected.
func TestDetectNodeWithoutStartScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"nuxt build"},"engines":{"node":">=22"}}`), 0644)
	env := &config.Environment{}
	env.Builds.Frontend.Enabled = true
	if b := DetectBackends(env, dir); len(b) != 0 {
		t.Fatalf("a frontend that is only compiled is no backend: %+v", b)
	}
	env.Services = []config.ServiceConfig{{Name: "site", Exec: "node .output/server/index.mjs"}}
	if b := DetectBackends(env, dir); len(b) != 1 || b[0].Kind != "node" || b[0].Detail != ">=22" {
		t.Fatalf("node service: %+v", b)
	}
	env.Services = nil
	env.Provision.NodeMajor = "22"
	if b := DetectBackends(env, dir); len(b) != 1 || b[0].Kind != "node" {
		t.Fatalf("node_major: %+v", b)
	}
}

// PHP-FPM versions, FrankenPHP next to PHP-FPM and Node majors are per project: never
// refused on Debian/Ubuntu. RHEL has one php-fpm; real alternatives stay refused.
func TestConflictsPerProjectVersions(t *testing.T) {
	apt := ParseReport("apt=1\nphp_fpm=8.4\nsvc_php=active 8.4.1\nsvc_frankenphp=active v1.13\nsvc_node=installed v22.1.0\nsvc_apache=active 2.4")
	v := Vars{PHPVersion: "8.1", NodeMajor: "20", WebServer: "nginx"}
	for _, ids := range [][]string{{"php"}, {"frankenphp"}, {"node"}} {
		if err := Conflicts(apt, ids, v); err != nil {
			t.Errorf("%v next to other versions: %v", ids, err)
		}
	}
	if Conflicts(apt, []string{"nginx"}, v) == nil {
		t.Error("nginx next to Apache must stay refused")
	}
	if Conflicts(apt, []string{"php", "frankenphp"}, v) == nil {
		t.Error("PHP-FPM and FrankenPHP for the same app must be refused")
	}
	rhel := ParseReport("dnf=1\nphp_fpm=8.3\nsvc_php=active 8.3.1")
	if err := Conflicts(rhel, []string{"php"}, v); err == nil || !strings.Contains(err.Error(), "FrankenPHP") {
		t.Errorf("RHEL: a second PHP-FPM version must be refused, pointing to FrankenPHP: %v", err)
	}
	if err := Conflicts(rhel, []string{"frankenphp"}, v); err != nil {
		t.Errorf("RHEL: FrankenPHP next to PHP-FPM: %v", err)
	}
}

// The probe reports the project's PHP and Node, not another project's: 8.4 and Node 22 on
// the server, the project wants 8.1 and 20 → both absent (preselected), and the bad
// values never reach the shell.
func TestDetectScriptProjectVersions(t *testing.T) {
	s := DetectScript(DetectParams{Project: "a", RemotePath: "/var/www/a", PHPVersion: "8.1", NodeMajor: "20"})
	if !strings.Contains(s, "phpv='8.1'; nodem='20'") {
		t.Errorf("versions not passed: %s", s[:200])
	}
	s = DetectScript(DetectParams{PHPVersion: "8.1; rm -rf /", NodeMajor: "$(id)"})
	if !strings.Contains(s, "phpv=''; nodem=''") {
		t.Errorf("malformed versions must be dropped: %s", s[:200])
	}
}
