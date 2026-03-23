package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/config"
)

func TestCreateHostKeyCallback(t *testing.T) {
	// Case 1: Insecure backup when no known_hosts found
	cfg := &config.SSHConfig{KnownHostsFile: "non-existent-file"}
	callback := createHostKeyCallback(cfg)
	if callback == nil {
		t.Error("expected a callback, got nil")
	}

	// Case 2: Explicit known_hosts file
	tmpFile := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(tmpFile, []byte(""), 0644)
	cfg2 := &config.SSHConfig{KnownHostsFile: tmpFile}
	callback2 := createHostKeyCallback(cfg2)
	if callback2 == nil {
		t.Error("expected a callback for existing file, got nil")
	}

	// Case 3: Empty path should try default (cannot easily mock home, but can check it doesn't panic)
	cfg3 := &config.SSHConfig{KnownHostsFile: ""}
	createHostKeyCallback(cfg3)
}

func TestParseDiskSpace(t *testing.T) {
	// CheckDiskSpace uses c.ExecuteCommand which we can't easily mock here without refactor.
	// But we can test the internal logic if we isolate it.
}
