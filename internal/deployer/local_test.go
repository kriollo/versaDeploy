package deployer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

func setupLocalTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runCmd("init")
	runCmd("config", "user.email", "test@example.com")
	runCmd("config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repoDir, "index.php"), []byte("<?php echo 'hi';"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd("add", "index.php")
	runCmd("commit", "-m", "initial commit")

	return repoDir
}

func newLocalTestDeployer(t *testing.T, repoDir, localPath string, postDeploy []config.HookConfig) *Deployer {
	t.Helper()
	cfg := &config.Config{
		Project: "test-local",
		Environments: map[string]config.Environment{
			"local": {
				Local:      true,
				LocalPath:  localPath,
				PostDeploy: postDeploy,
			},
		},
	}
	log, _ := logger.NewLogger("", false, false)
	d, err := NewDeployer(cfg, "local", repoDir, false, false, false, true, log)
	if err != nil {
		t.Fatalf("NewDeployer failed: %v", err)
	}
	return d
}

func TestDeployLocal_FullReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hook execution via sh -c is not supported on windows")
	}

	repoDir := setupLocalTestRepo(t)
	localPath := filepath.Join(t.TempDir(), "output")

	// Pre-existing stale file that should be gone after the deploy replaces everything.
	if err := os.MkdirAll(localPath, 0775); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(localPath, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	d := newLocalTestDeployer(t, repoDir, localPath, nil)
	if err := d.Deploy(); err != nil {
		t.Fatalf("Deploy() failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(localPath, "app", "index.php")); err != nil {
		t.Errorf("expected app/index.php to be present in %s: %v", localPath, err)
	}
	if _, err := os.Stat(filepath.Join(localPath, "manifest.json")); err != nil {
		t.Errorf("expected manifest.json to be present in %s: %v", localPath, err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("expected stale.txt to be removed by full replace, got err=%v", err)
	}
}

func TestDeployLocal_PostDeployHookRunsInLocalPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hook execution via sh -c is not supported on windows")
	}

	repoDir := setupLocalTestRepo(t)
	localPath := filepath.Join(t.TempDir(), "output")

	hooks := []config.HookConfig{{Command: "pwd > hook_cwd.txt"}}
	d := newLocalTestDeployer(t, repoDir, localPath, hooks)

	if err := d.Deploy(); err != nil {
		t.Fatalf("Deploy() failed: %v", err)
	}

	wantCwd := filepath.Join(localPath, "app")
	data, err := os.ReadFile(filepath.Join(wantCwd, "hook_cwd.txt"))
	if err != nil {
		t.Fatalf("expected hook_cwd.txt written by post_deploy hook: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if got != wantCwd {
		t.Errorf("expected hook cwd to be %s, got %q", wantCwd, got)
	}
}

func TestDeployer_Rollback_UnsupportedForLocal(t *testing.T) {
	repoDir := t.TempDir()
	d := newLocalTestDeployer(t, repoDir, filepath.Join(t.TempDir(), "output"), nil)

	if err := d.Rollback(); err == nil {
		t.Error("expected Rollback() to fail for local environment")
	}
	if err := d.Status(); err == nil {
		t.Error("expected Status() to fail for local environment")
	}
	if _, err := d.ExecRemoteCommand("echo hi"); err == nil {
		t.Error("expected ExecRemoteCommand() to fail for local environment")
	}
}
