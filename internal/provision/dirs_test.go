package provision

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runDirs runs scripts/dirs.sh with chown/chmod stubbed to log the directory they
// would change (pwd -P), so the test needs no root.
func runDirs(t *testing.T, remotePath string) (string, error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs bash")
	}
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	body, _ := assets.ReadFile("scripts/dirs.sh")
	log := filepath.Join(t.TempDir(), "log")
	script := `set -euo pipefail
die() { echo "[ERROR] $*"; exit 1; }
ok() { echo "[OK] $*"; }
chown() { echo "chown $(pwd -P) $*" >> "$LOG"; }
chmod() { echo "chmod $(pwd -P) $*" >> "$LOG"; }
selinux_label() { :; }
` + string(body)
	cmd := exec.Command(bash, "-c", script)
	cmd.Env = append(os.Environ(), "LOG="+log, "DEPLOY_USER="+u.Username, "REMOTE_PATH="+remotePath)
	out, err := cmd.CombinedOutput()
	got, _ := os.ReadFile(log)
	t.Logf("%s", out)
	return string(got), err
}

func TestDirsScriptRefusesSymlinks(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	victim := filepath.Join(root, "victim")
	os.Mkdir(victim, 0700)

	real := filepath.Join(root, "app")
	got, err := runDirs(t, real)
	if err != nil || !strings.Contains(got, "chown "+real+" ") || !strings.Contains(got, "chmod "+real+" 2755") {
		t.Fatalf("real path: err=%v log=%q", err, got)
	}

	link := filepath.Join(root, "link")
	os.Symlink(victim, link)
	if got, err := runDirs(t, link); err == nil || got != "" {
		t.Fatalf("symlinked remote_path must be refused: err=%v log=%q", err, got)
	}

	os.Symlink(victim, filepath.Join(root, "parent"))
	if got, err := runDirs(t, filepath.Join(root, "parent", "app")); err == nil || got != "" {
		t.Fatalf("symlinked parent must be refused: err=%v log=%q", err, got)
	}
}

// A compromised server controls the detection report; its values reach root scripts.
func TestUpgradeIgnoresHostileReportValues(t *testing.T) {
	r := ParseReport("apt=1\nphp_fpm=8.4';id;'\nupg php8.4';id;'-fpm/noble-updates 1 amd64 [upgradable from: 0]\nupg -oAPT::x=1/noble-updates 1 amd64\nupg nginx/noble-security 1.2 amd64 [upgradable from: 1.1]")
	if r.PHPVersion != "" {
		t.Errorf("php version accepted: %q", r.PHPVersion)
	}
	if len(r.Upgrades) != 1 || r.Upgrades[0].Pkg != "nginx" {
		t.Errorf("upgrades = %+v", r.Upgrades)
	}
	run := string(UpgradeBundle([]string{"nginx", "x';id;'", "-oAPT::x=1"}, []string{"nginx", "php8.4';id;'-fpm"}, false)["run.sh"])
	for _, bad := range []string{"id;", "-oAPT"} {
		if strings.Contains(run, bad) {
			t.Errorf("run.sh contains %q:\n%s", bad, run)
		}
	}
	if !strings.Contains(run, "'nginx'") {
		t.Error("valid package dropped")
	}
}
