package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	repoDir := t.TempDir()

	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run %s %v: %v", name, args, err)
		}
	}

	runCmd("git", "init")
	runCmd("git", "config", "user.email", "test@example.com")
	runCmd("git", "config", "user.name", "Test User")

	os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("hello"), 0644)
	runCmd("git", "add", "file.txt")
	runCmd("git", "commit", "-m", "initial commit")

	return repoDir
}

func TestValidateRepository(t *testing.T) {
	repoDir := setupGitRepo(t)

	if err := ValidateRepository(repoDir); err != nil {
		t.Errorf("ValidateRepository() error = %v, want nil", err)
	}

	invalidDir := t.TempDir()
	if err := ValidateRepository(invalidDir); err == nil {
		t.Error("ValidateRepository() error = nil, want error")
	}
}

func TestGetCurrentCommit(t *testing.T) {
	repoDir := setupGitRepo(t)

	commit, err := GetCurrentCommit(repoDir)
	if err != nil {
		t.Fatalf("GetCurrentCommit() error = %v", err)
	}

	if len(commit) != 40 {
		t.Errorf("expected 40 chars commit hash, got %d", len(commit))
	}
}

func TestIsClean(t *testing.T) {
	repoDir := setupGitRepo(t)

	clean, err := IsClean(repoDir)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Error("expected repo to be clean")
	}

	os.WriteFile(filepath.Join(repoDir, "dirty.txt"), []byte("dirty"), 0644)
	clean, err = IsClean(repoDir)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if clean {
		t.Error("expected repo to be dirty after creating new file")
	}
}

func TestClone(t *testing.T) {
	repoDir := setupGitRepo(t)

	tmpDir, err := Clone(repoDir, "")
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := os.Stat(filepath.Join(tmpDir, "file.txt")); os.IsNotExist(err) {
		t.Error("cloned repo is missing file.txt")
	}
}

func TestClone_WithRef(t *testing.T) {
	repoDir := setupGitRepo(t)

	// Create a branch
	cmd := exec.Command("git", "-C", repoDir, "checkout", "-b", "feature")
	cmd.Run()
	os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature"), 0644)
	cmd = exec.Command("git", "-C", repoDir, "add", "feature.txt")
	cmd.Run()
	cmd = exec.Command("git", "-C", repoDir, "commit", "-m", "feature commit")
	cmd.Run()

	tmpDir, err := Clone(repoDir, "feature")
	if err != nil {
		t.Fatalf("Clone(feature) error = %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := os.Stat(filepath.Join(tmpDir, "feature.txt")); os.IsNotExist(err) {
		t.Error("cloned repo with branch feature is missing feature.txt")
	}
}

func TestClone_Fail(t *testing.T) {
	_, err := Clone("/invalid/path", "")
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}

func TestGetCurrentCommit_Fail(t *testing.T) {
	_, err := GetCurrentCommit("/invalid/path")
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}
