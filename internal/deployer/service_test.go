package deployer

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

func serviceEnv(remote string) *config.Environment {
	return &config.Environment{
		SSH:        config.SSHConfig{User: "deploy"},
		RemotePath: remote,
		Builds: config.BuildsConfig{Go: config.GoBuildConfig{
			Enabled: true, DeployPath: "bin/go", BinaryName: "api",
		}},
		Services: []config.ServiceConfig{{
			Name: "api", User: "deploy", EnvFile: ".env", StartWait: 1, StopTimeout: 30,
			Environment: map[string]string{"PORT": "8080", "MSG": "it's ok"},
		}},
	}
}

func TestRenderService(t *testing.T) {
	env := serviceEnv("/var/www/my app")
	files, err := renderService("shop", "production", env, env.Services[0], env.RemotePath, serverLayout)
	if err != nil {
		t.Fatal(err)
	}

	wrapper := files["api.sh"]
	for _, want := range []string{
		"cd '/var/www/my app/current/app' || exit 1",
		`[ -f '/var/www/my app/shared/.env' ]; then set -a; . '/var/www/my app/shared/.env'; set +a; fi`,
		"export MSG='it'\\''s ok'\nexport PORT='8080'\n",
		"exec /var/www/my app/current/bin/go/api\n",
	} {
		if !strings.Contains(wrapper, want) {
			t.Errorf("wrapper missing %q:\n%s", want, wrapper)
		}
	}

	unit := files["api.service"]
	for _, want := range []string{
		"User=deploy",
		"EnvironmentFile=-/var/www/my app/shared/.env",
		"Environment=VERSA_ENV_LOADED=1",
		`ExecStart=/bin/sh "/var/www/my app/.versa/api.sh"`,
		"Restart=always",
		"TimeoutStopSec=30",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q:\n%s", want, unit)
		}
	}

	if !strings.Contains(files["api.initd"], "# chkconfig: 2345") || !strings.Contains(files["api.initd"], "# Provides:          api") {
		t.Errorf("SysV script lacks chkconfig/LSB headers:\n%s", files["api.initd"])
	}
	if !strings.Contains(files["api.initd"], "STOP_TIMEOUT=30\n") {
		t.Errorf("SysV script ignores stop_timeout:\n%s", files["api.initd"])
	}
	if o := files["api.openrc"]; !strings.HasPrefix(o, "#!/sbin/openrc-run") || !strings.Contains(o, "command_user='deploy'") ||
		!strings.Contains(o, `retry="TERM/30/KILL/5"`) || !strings.Contains(o, "pidfile='/var/run/api.pid'") {
		t.Errorf("unexpected OpenRC script:\n%s", files["api.openrc"])
	}
}

func TestServiceExec(t *testing.T) {
	env := serviceEnv("/srv/app")
	cases := map[string]string{
		"":                      "/srv/app/current/bin/go/api",
		"./bin/go/api --port 1": "/srv/app/current/bin/go/api --port 1",
		"/usr/bin/node app.js":  "/usr/bin/node app.js",
	}
	for exec, want := range cases {
		got, err := serviceExec(env, config.ServiceConfig{Exec: exec}, "/srv/app/current")
		if err != nil || got != want {
			t.Errorf("serviceExec(%q) = %q, %v; want %q", exec, got, err, want)
		}
	}
	if wd := serviceWorkDir(env, config.ServiceConfig{WorkingDir: "app/api/"}, "/srv/app/current"); wd != "/srv/app/current/app/api" {
		t.Errorf("work dir = %q", wd)
	}
}

// stubBin writes executable sh scripts into a temp dir to put first in PATH.
func stubBin(t *testing.T, stubs map[string]string) string {
	dir := t.TempDir()
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runScript(t *testing.T, script, path string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+path+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func requireSh(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
}

// systemd stubs: state lives in marker files so is-enabled / is-active reflect calls.
func systemdStubs(t *testing.T, state string) string {
	return stubBin(t, map[string]string{
		"sudo": `echo "sudo $*" >>"` + state + `/log"; [ "$1" = -n ] && shift; exec "$@"`,
		"systemctl": `case "$1" in
is-enabled) [ -f "` + state + `/enabled" ] ;;
enable) touch "` + state + `/enabled" ;;
is-active) [ -f "` + state + `/active" ] ;;
restart|start) [ -f "` + state + `/broken" ] || touch "` + state + `/active" ;;
status) echo "stub status of $3" ;;
*) : ;;
esac`,
	})
}

func TestServiceInstallSystemd(t *testing.T) {
	requireSh(t)
	remote, state := t.TempDir(), t.TempDir()
	env := serviceEnv(filepath.ToSlash(remote))
	l := svcLayout{etc: filepath.ToSlash(filepath.Join(remote, "etc")), run: filepath.ToSlash(remote), log: filepath.ToSlash(remote)}
	os.MkdirAll(filepath.Join(remote, "etc", "systemd", "system"), 0755)
	os.MkdirAll(filepath.Join(remote, ".versa"), 0755)
	files, _ := renderService("shop", "prod", env, env.Services[0], env.RemotePath, l)
	for name, content := range files {
		os.WriteFile(filepath.Join(remote, ".versa", name), []byte(content), 0644)
	}
	bin := systemdStubs(t, filepath.ToSlash(state))
	script := serviceScript(env, "install", nil, 0, l, "I=systemd\n")

	out, err := runScript(t, script, bin)
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}
	log, _ := os.ReadFile(filepath.Join(state, "log"))
	for _, want := range []string{"install -m 644", "systemctl daemon-reload", "systemctl enable api"} {
		// sudo gets absolute paths (sudoers matches them): "sudo -n /usr/bin/install -m 644 ..."
		if !regexp.MustCompile(`sudo -n /\S*` + regexp.QuoteMeta(want)).MatchString(string(log)) {
			t.Errorf("first install didn't run %q; log:\n%s", want, log)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(remote, "etc", "systemd", "system", "api.service")); string(b) != files["api.service"] {
		t.Error("unit not copied to the systemd directory")
	}

	// Unchanged unit, already enabled: no sudo at all, so a restricted sudoers works
	os.Remove(filepath.Join(state, "log"))
	if out, err := runScript(t, script, bin); err != nil {
		t.Fatalf("second install failed: %v\n%s", err, out)
	}
	if log, err := os.ReadFile(filepath.Join(state, "log")); err == nil {
		t.Errorf("second install used sudo:\n%s", log)
	}
}

func TestServiceRestartVerifies(t *testing.T) {
	requireSh(t)
	state := t.TempDir()
	env := serviceEnv("/srv/app")
	bin := systemdStubs(t, filepath.ToSlash(state))
	script := serviceScript(env, "restart", nil, 0, serverLayout, "I=systemd\n")

	if out, err := runScript(t, script, bin); err != nil {
		t.Fatalf("restart of a healthy service failed: %v\n%s", err, out)
	}

	os.Remove(filepath.Join(state, "active"))
	os.WriteFile(filepath.Join(state, "broken"), nil, 0644)
	out, err := runScript(t, script, bin)
	if err == nil || !strings.Contains(out, "api is not running") {
		t.Fatalf("a service that doesn't stay up must fail the restart: %v\n%s", err, out)
	}
}

// TestSysVInitScript starts, checks and stops a real process with the generated SysV script.
func TestSysVInitScript(t *testing.T) {
	requireSh(t)
	if runtime.GOOS == "windows" {
		t.Skip("init scripts run on the Linux server; process control under Git Bash differs")
	}
	remote := t.TempDir()
	user := os.Getenv("USER")
	if user == "" {
		t.Skip("USER not set")
	}
	env := serviceEnv(remote)
	env.Services[0].User = user
	env.Services[0].Exec = "sleep 60"
	l := svcLayout{etc: remote, run: remote, log: remote}
	os.MkdirAll(filepath.Join(remote, "current", "app"), 0755)
	os.MkdirAll(filepath.Join(remote, ".versa"), 0755)
	files, _ := renderService("shop", "prod", env, env.Services[0], remote, l)
	for name, content := range files {
		os.WriteFile(filepath.Join(remote, ".versa", name), []byte(content), 0755)
	}
	initd := filepath.Join(remote, ".versa", "api.initd")
	run := func(action string) (string, error) {
		out, err := exec.Command("sh", initd, action).CombinedOutput()
		return string(out), err
	}

	if out, err := run("start"); err != nil || !strings.Contains(out, "api started") {
		t.Fatalf("start: %v %s", err, out)
	}
	if out, err := run("status"); err != nil || !strings.Contains(out, "is running") {
		t.Fatalf("status after start: %v %s", err, out)
	}
	if out, err := run("restart"); err != nil || !strings.Contains(out, "api started") {
		t.Fatalf("restart: %v %s", err, out)
	}
	if out, err := run("stop"); err != nil || !strings.Contains(out, "api stopped") {
		t.Fatalf("stop: %v %s", err, out)
	}
	out, err := run("status")
	if err == nil || !strings.Contains(out, "not running") {
		t.Fatalf("status after stop must exit non-zero: %v %s", err, out)
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 3 {
		t.Errorf("status of a stopped service should exit 3 (LSB), got %v", err)
	}
}

func TestSudoersScript(t *testing.T) {
	requireSh(t)
	env := serviceEnv("/srv/app")
	out, err := runScript(t, sudoersScript(env, "my shop"), "")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "deploy ALL=(root) NOPASSWD:") || !strings.Contains(out, " api") || !strings.Contains(out, "Defaults:deploy !requiretty") || !strings.Contains(out, "versa services for my shop") {
		t.Errorf("unexpected sudoers output:\n%s", out)
	}
}

func TestSudoPrefix(t *testing.T) {
	if p := sudoPrefix(&config.Environment{SSH: config.SSHConfig{User: "root"}}); p != "" {
		t.Errorf("root must not use sudo, got %q", p)
	}
	if p := sudoPrefix(&config.Environment{SSH: config.SSHConfig{User: "deploy"}}); p != "sudo -n " {
		t.Errorf("non-root must use non-interactive sudo, got %q", p)
	}
}

func TestVenvScript(t *testing.T) {
	requireSh(t)
	app, state := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(app, "requirements.txt"), []byte("flask\n"), 0644)
	// python3 -m venv DIR creates a stub interpreter that logs what it is asked to run
	bin := stubBin(t, map[string]string{"python3": `if [ "$1 $2" = "-m venv" ]; then
mkdir -p "$3/bin"
printf '#!/bin/sh\necho "venv-python $*" >>"` + filepath.ToSlash(state) + `/log"\n' >"$3/bin/python"
chmod +x "$3/bin/python"
fi`})
	py := config.PythonBuildConfig{PythonCommand: "python3", PackageManager: "pip", RequirementsFile: "requirements.txt", VenvPath: ".venv", PyPIMirror: "https://mirror.example/simple"}
	script := venvScript(py, filepath.ToSlash(app))

	out, err := runScript(t, script, bin)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	log, _ := os.ReadFile(filepath.Join(state, "log"))
	if !strings.Contains(string(log), "venv-python -m pip install -r requirements.txt --no-cache-dir -i https://mirror.example/simple") {
		t.Errorf("pip not run in the venv as expected:\n%s\n%s", log, out)
	}

	out, err = runScript(t, script, bin)
	if err != nil || strings.TrimSpace(out) != "reused" {
		t.Errorf("an existing venv must be reused: %v %q", err, out)
	}
}

func TestVenvScriptMissingPython(t *testing.T) {
	requireSh(t)
	py := config.PythonBuildConfig{PythonCommand: "python-that-does-not-exist", PackageManager: "pip", RequirementsFile: "requirements.txt", VenvPath: ".venv"}
	out, err := runScript(t, venvScript(py, filepath.ToSlash(t.TempDir())), "")
	if err == nil || !strings.Contains(out, "not found on the server") {
		t.Errorf("expected a clear error, got %v %q", err, out)
	}
}

// ServiceAction rejects bad input before opening any SSH connection.
func TestServiceActionValidation(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	two := serviceEnv("/srv/app")
	two.Services = append(two.Services, config.ServiceConfig{Name: "worker", Exec: "./bin/go/api work"})
	cases := []struct {
		env          *config.Environment
		action, name string
		want         string
	}{
		{&config.Environment{RemotePath: "/srv/app"}, "status", "", "has no services"},
		{serviceEnv("/srv/app"), "reboot", "", "unknown action"},
		{serviceEnv("/srv/app"), "status", "nope", "is not defined"},
		{two, "logs", "", "choose one with --name"},
		{&config.Environment{Local: true}, "status", "", "not supported for local"},
	}
	for _, tc := range cases {
		d := &Deployer{cfg: &config.Config{Project: "p"}, env: tc.env, envName: "prod", log: log}
		if err := d.ServiceAction(tc.action, tc.name, 10, io.Discard); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s/%s: want error containing %q, got %v", tc.action, tc.name, tc.want, err)
		}
	}
}
