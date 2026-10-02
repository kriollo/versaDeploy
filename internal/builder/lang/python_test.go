package lang

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

func TestRunServerScript(t *testing.T) {
	base := config.PythonBuildConfig{VenvPath: ".venv", WebHost: "0.0.0.0", WebPort: 8000, EntryPoint: "api/main.py"}
	with := func(f func(*config.PythonBuildConfig)) config.PythonBuildConfig {
		c := base
		f(&c)
		return c
	}
	cases := []struct {
		name string
		cfg  config.PythonBuildConfig
		wsgi string
		want string
	}{
		{"fastapi uses the venv", with(func(c *config.PythonBuildConfig) { c.WebFramework = "fastapi"; c.WebWorkers = 2 }), "",
			"exec .venv/bin/python -m uvicorn api.main:app --host 0.0.0.0 --port 8000 --workers 2\n"},
		{"gunicorn", with(func(c *config.PythonBuildConfig) { c.WebFramework = "gunicorn"; c.WebThreads = 4 }), "",
			"exec .venv/bin/python -m gunicorn api.main:app -w 4 -b 0.0.0.0:8000 --threads 4\n"},
		{"django with wsgi", with(func(c *config.PythonBuildConfig) { c.WebFramework = "django" }), "mysite.wsgi",
			"exec .venv/bin/python -m gunicorn mysite.wsgi:application -w 2 -b 0.0.0.0:8000\n"},
		{"django without wsgi", with(func(c *config.PythonBuildConfig) { c.WebFramework = "django" }), "",
			"exec .venv/bin/python manage.py runserver 0.0.0.0:8000 --noreload\n"},
		{"run_command wins", with(func(c *config.PythonBuildConfig) { c.WebFramework = "django"; c.RunCommand = "./start.sh" }), "x.wsgi",
			"exec ./start.sh\n"},
	}
	for _, tc := range cases {
		got := runServerScript(tc.cfg, tc.wsgi)
		if !strings.HasPrefix(got, "#!/bin/sh\n") || !strings.Contains(got, `cd "$(dirname "$0")" || exit 1`) || !strings.HasSuffix(got, tc.want) {
			t.Errorf("%s:\n%s\nwant suffix %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, "migrate") {
			t.Errorf("%s: migrations belong in post_deploy hooks, not in the start script", tc.name)
		}
	}
}

func TestFindDjangoWSGI(t *testing.T) {
	dir := t.TempDir()
	if got := findDjangoWSGI(dir); got != "" {
		t.Errorf("empty project: %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "mysite"), 0755)
	os.WriteFile(filepath.Join(dir, "mysite", "wsgi.py"), nil, 0644)
	if got := findDjangoWSGI(dir); got != "mysite.wsgi" {
		t.Errorf("got %q, want mysite.wsgi", got)
	}
}

// The build writes run_server.sh even when no Python file changed, and never installs
// packages locally (the venv is created on the server).
func TestPythonBuildWritesRunScriptOnly(t *testing.T) {
	out := t.TempDir()
	log, _ := logger.NewLogger("", false, false)
	ctx := &BuilderContext{
		ArtifactDir: out,
		Config: &config.Environment{Builds: config.BuildsConfig{Python: config.PythonBuildConfig{
			Enabled: true, WebServer: true, WebFramework: "fastapi", EntryPoint: "main.py", VenvPath: ".venv",
			WebHost: "0.0.0.0", WebPort: 8000, PythonCommand: "python-that-does-not-exist", RequirementsFile: "requirements.txt",
		}}},
		Changeset: &changeset.ChangeSet{RequirementsChanged: true},
		Log:       log,
	}
	if _, _, err := (&PythonBuilder{}).Build(ctx); err != nil {
		t.Fatalf("build must not run pip locally: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "app", "run_server.sh")); err != nil {
		t.Errorf("run_server.sh not written: %v", err)
	}
}

func TestPythonBuildBinaryNeedsLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("PyInstaller builds are allowed on Linux")
	}
	log, _ := logger.NewLogger("", false, false)
	ctx := &BuilderContext{
		ArtifactDir: t.TempDir(),
		Config: &config.Environment{Builds: config.BuildsConfig{Python: config.PythonBuildConfig{
			Enabled: true, BuildBinary: true, EntryPoint: "main.py", BinaryName: "app",
		}}},
		Changeset: &changeset.ChangeSet{Force: true},
		Log:       log,
	}
	if _, _, err := (&PythonBuilder{}).Build(ctx); err == nil || !strings.Contains(err.Error(), "needs versa to run on Linux") {
		t.Errorf("expected a clear error, got %v", err)
	}
}
