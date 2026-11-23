package provision

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
)

func TestComputeTuning(t *testing.T) {
	for _, c := range []struct {
		profile              string
		cpus, mem            int
		threads, shared, eff int
	}{
		{"small", 2, 4096, 4, 614, 2048},
		{"auto", 4, 8192, 8, 1228, 4096},
		{"event", 4, 8192, 16, 1228, 4096},
		{"event", 6, 16384, 24, 2457, 8192}, // the 16 GB / 6 vCPU VPS
		{"event", 1, 1024, 4, 153, 512},     // RAM caps the workers, not the CPU
	} {
		got := ComputeTuning(config.TuningConfig{Profile: c.profile}, c.cpus, c.mem)
		if got.MaxThreads != c.threads || got.SharedBuffersMB != c.shared || got.EffectiveCacheMB != c.eff {
			t.Errorf("%s %d/%d: %d threads, %d shared, %d cache", c.profile, c.cpus, c.mem, got.MaxThreads, got.SharedBuffersMB, got.EffectiveCacheMB)
		}
	}
	half := ComputeTuning(config.TuningConfig{Profile: "event", Share: "50%"}, 6, 16384)
	if half.SharedBuffersMB != 1228 {
		t.Errorf("share ignored: %d", half.SharedBuffersMB)
	}
	if ComputeTuning(config.TuningConfig{}, 6, 16384).Active || ComputeTuning(config.TuningConfig{Profile: "off"}, 6, 16384).Active {
		t.Error("tuning must be off by default")
	}
}

// Without tuning nothing about a project's PHP pool, proxy or threads changes.
func TestTuningOffKeepsConfig(t *testing.T) {
	for _, proxy := range []int{0, 8000} {
		v := testVars()
		v.ProxyPort = proxy
		out, err := RenderVhost(v, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, no := range []string{"upstream ", "$versa_connection", "-fpm-versatodo"} {
			if strings.Contains(out, no) {
				t.Errorf("proxy %d: %q appears with tuning off", proxy, no)
			}
		}
	}
	v := testVars()
	v.FrankenPHP, v.ProxyPort, v.AppService = true, 8000, "app"
	c, err := renderCaddyfile(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(c), "max_threads") || !strings.Contains(string(c), "trusted_proxies_strict") {
		t.Errorf("Caddyfile:\n%s", c)
	}
}

func TestTunedRender(t *testing.T) {
	v := testVars()
	v.Tuning = config.TuningConfig{Profile: "event", UploadMax: "1G"}
	v.Tune = ComputeTuning(v.Tuning, 6, 16384)
	fpm, err := RenderVhost(v, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"client_max_body_size 1G;", "fastcgi_pass unix:/run/php/php8.4-fpm-versatodo.sock;"} {
		if !strings.Contains(fpm, want) {
			t.Errorf("missing %q", want)
		}
	}
	v.FrankenPHP, v.ProxyPort, v.AppService = true, 8000, "app"
	proxy, _ := RenderVhost(v, false)
	for _, want := range []string{"upstream versa_app_versatodo {", "keepalive 64;", "proxy_pass http://versa_app_versatodo$request_uri;", "Connection $versa_connection"} {
		if !strings.Contains(proxy, want) {
			t.Errorf("missing %q", want)
		}
	}
	c, _ := renderCaddyfile(v)
	for _, want := range []string{"num_threads 6", "max_threads 24", "max_wait_time 15s", "upload_max_filesize 1G"} {
		if !strings.Contains(string(c), want) {
			t.Errorf("Caddyfile lacks %q", want)
		}
	}
	apache := testVars()
	apache.WebServer, apache.Tuning = "apache", config.TuningConfig{UploadMax: "1G"}
	a, _ := RenderVhost(apache, false)
	if !strings.Contains(a, "LimitRequestBody 1073741824") {
		t.Error("Apache ignores upload_max")
	}
}

func TestTuneBundleRestartsOnlyWhatIsNamed(t *testing.T) {
	v := testVars()
	v.FrankenPHP, v.ProxyPort, v.AppService = true, 8000, "app"
	v.Tuning = config.TuningConfig{Profile: "event"}
	v.Tune = ComputeTuning(v.Tuning, 6, 16384)
	if _, err := TuneBundle(v, []string{"sshd"}); err == nil {
		t.Error("restarted a service it doesn't own")
	}
	b, err := TuneBundle(v, []string{"app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b["env"]), "TUNE_RESTART='app'") || b["Caddyfile"] == nil {
		t.Errorf("bundle: %s", b["env"])
	}
	f := t.TempDir() + "/run.sh"
	os.WriteFile(f, b["run.sh"], 0o600)
	if out, err := exec.Command("bash", "-n", f).CombinedOutput(); err != nil {
		t.Fatalf("syntax: %s", out)
	}
}

func TestRestoreRefusesOtherFiles(t *testing.T) {
	v := testVars()
	for _, f := range []string{"../../etc/shadow", "otra-20260101-000000.sql.gz", "todo-20260101-000000.sql.gz;id"} {
		if _, err := RestoreScript(v, f, false); err == nil {
			t.Errorf("accepted %q", f)
		}
	}
	s, err := RestoreScript(v, "todo-20260101-000000.sql.gz", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "as sh -c 'gzip -dc") || strings.Contains(s, "DROP DATABASE IF EXISTS ${DB_NAME}") {
		t.Error("the dump must run as the deploy user and never drop the live database")
	}
	if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
		t.Fatalf("syntax: %s", out)
	}
}

func TestBackupSafety(t *testing.T) {
	v := testVars()
	v.BackupTime, v.Timezone, v.BackupUpload = "04:30", "America/Santiago", "aws s3 cp $BACKUP_FILE s3://x/"
	b, err := Build([]string{"backup"}, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Nice=19", "IOSchedulingClass=idle", "gzip -t", `OnCalendar=${calendar}`} {
		if !strings.Contains(string(b["run.sh"]), want) {
			t.Errorf("missing %q", want)
		}
	}
	v.BackupTime = "25:00"
	if v.Validate([]string{"backup"}) == nil {
		t.Error("accepted an invalid time")
	}
}
