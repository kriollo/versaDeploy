package provision

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
		all = append(all, s.ID)
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
	sel := Preselect(got, v, py, &Report{Services: map[string]State{"nginx": {Status: Active}}})
	if !slices.Contains(sel, "python") || slices.Contains(sel, "app_service") || slices.Contains(sel, "nginx") {
		t.Errorf("preselect: %v", sel)
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
	if Conflicts(r, []string{"php"}, Vars{PHPVersion: "8.3"}) == nil {
		t.Error("second PHP-FPM version allowed")
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
	if Conflicts(r, []string{"node"}, Vars{NodeMajor: "22"}) == nil || Conflicts(r, []string{"node"}, Vars{NodeMajor: "20"}) != nil {
		t.Error("node major conflict check wrong")
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
