package deployer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/versaDeploy/internal/ssh"
)

// runShells runs script under sh and, when installed, busybox sh (Alpine, old embedded).
func runShells(t *testing.T, setup func(root string), script func(root string) string, check func(root, out string)) {
	shells := [][]string{{"sh", "-c"}}
	if _, err := exec.LookPath("busybox"); err == nil {
		shells = append(shells, []string{"busybox", "sh", "-c"})
	}
	for _, sh := range shells {
		t.Run(sh[0], func(t *testing.T) {
			root := t.TempDir()
			setup(root)
			out, err := exec.Command(sh[0], append(sh[1:], script(root))...).CombinedOutput()
			if err != nil {
				t.Fatalf("script failed: %v: %s", err, out)
			}
			check(root, string(out))
		})
	}
}

func TestSharedLinkScript(t *testing.T) {
	requireRealSymlinks(t)
	runShells(t, func(root string) {
		os.MkdirAll(filepath.Join(root, "rel/app/storage/logs"), 0755)
		os.MkdirAll(filepath.Join(root, "shared"), 0755)
		os.WriteFile(filepath.Join(root, "shared/.env"), []byte("KEEP"), 0644)
		os.WriteFile(filepath.Join(root, "rel/app/.env"), []byte("artifact"), 0644)
	}, func(root string) string {
		return sharedLinkScript(root+"/rel", root+"/shared", []string{"storage", ".env", "public/up loads", "../escape"})
	}, func(root, _ string) {
		for _, p := range []string{"storage", ".env", "public/up loads"} {
			if target, err := os.Readlink(filepath.Join(root, "rel/app", p)); err != nil || target != root+"/shared/"+p {
				t.Errorf("%s -> %q, %v", p, target, err)
			}
		}
		if b, _ := os.ReadFile(filepath.Join(root, "shared/.env")); string(b) != "KEEP" {
			t.Errorf("shared .env overwritten: %q", b)
		}
	})
}

func TestReuseSnippet(t *testing.T) {
	runShells(t, func(root string) {
		os.MkdirAll(filepath.Join(root, "old/vendor/pkg"), 0755)
		os.WriteFile(filepath.Join(root, "old/vendor/pkg/a.php"), []byte("x"), 0644)
		os.MkdirAll(filepath.Join(root, "new/kept"), 0755)
	}, func(root string) string {
		return strings.Join([]string{
			reuseSnippet(root+"/new/app/vendor", root+"/missing/vendor", root+"/old/vendor"),
			reuseSnippet(root+"/new/kept", root+"/old/vendor"),      // already present: untouched
			reuseSnippet(root+"/new/nothing", root+"/missing/none"), // no source: skipped
		}, "\n")
	}, func(root, out string) {
		if strings.TrimSpace(out) != root+"/new/app/vendor" {
			t.Errorf("output %q", out)
		}
		if _, err := os.Stat(filepath.Join(root, "new/app/vendor/pkg/a.php")); err != nil {
			t.Error("vendor not reused:", err)
		}
		if _, err := os.Stat(filepath.Join(root, "new/kept/pkg")); err == nil {
			t.Error("existing path was overwritten")
		}
	})
}

func TestPreserveScript(t *testing.T) {
	runShells(t, func(root string) {
		os.MkdirAll(filepath.Join(root, "prev/app/config"), 0755)
		os.WriteFile(filepath.Join(root, "prev/app/config/x.php"), []byte("server"), 0644)
		os.WriteFile(filepath.Join(root, "prev/legacy.txt"), []byte("legacy"), 0644)
		os.MkdirAll(filepath.Join(root, "final/app/config"), 0755)
		os.WriteFile(filepath.Join(root, "final/app/config/x.php"), []byte("artifact"), 0644)
	}, func(root string) string {
		return preserveScript(root+"/prev", root+"/final", []string{"config", "legacy.txt", "gone"})
	}, func(root, out string) {
		if want := "ok config\nok legacy.txt\nmissing gone"; strings.TrimSpace(out) != want {
			t.Errorf("output %q, want %q", out, want)
		}
		if b, _ := os.ReadFile(filepath.Join(root, "final/app/config/x.php")); string(b) != "server" {
			t.Errorf("config not restored: %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(root, "final/app/legacy.txt")); string(b) != "legacy" {
			t.Errorf("legacy path not restored: %q", b)
		}
	})
}

// The remote `timeout` wrapper must kill a hook that outlives hook_timeout.
func TestHookCmdTimeout(t *testing.T) {
	if exec.Command("timeout", "-s", "KILL", "5", "true").Run() != nil {
		t.Skip("no usable timeout")
	}
	d := &Deployer{server: &ssh.ServerInfo{Tools: []string{"timeout"}}}
	start := time.Now()
	cmd := exec.Command("sh", "-c", d.hookCmd(t.TempDir(), `echo "it's running"; sleep 10`, time.Second))
	cmd.Env = append(os.Environ(), "SHELL=/bin/sh")
	out, err := cmd.CombinedOutput()
	if err == nil || time.Since(start) > 5*time.Second || !strings.Contains(string(out), "it's running") {
		t.Errorf("err=%v after %v, output %q", err, time.Since(start), out)
	}
}

// requireRealSymlinks skips the test when this machine can't create real symlinks
// from Go and from sh (Windows without Developer Mode, where Git Bash's ln -s copies
// instead). The scripts under test only ever run on the Linux server.
func requireRealSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink("d", filepath.Join(dir, "go-link")); err != nil {
		t.Skipf("symlinks not supported here: %v", err)
	}
	cmd := exec.Command("sh", "-c", "mkdir d && ln -s d sh-link")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("sh can't create symlinks: %v: %s", err, out)
	}
	if _, err := os.Readlink(filepath.Join(dir, "sh-link")); err != nil {
		t.Skip("sh's ln -s doesn't create real symlinks here (Git Bash without Developer Mode)")
	}
}
