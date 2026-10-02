package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/user/versaDeploy/internal/config"
)

func TestCreateHostKeyCallback(t *testing.T) {
	// Case 1: Insecure backup when no known_hosts found
	cfg := &config.SSHConfig{KnownHostsFile: "non-existent-file"}
	callback, _ := createHostKeyCallback(cfg, nil)
	if callback == nil {
		t.Error("expected a callback, got nil")
	}

	// Case 2: Explicit known_hosts file
	tmpFile := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(tmpFile, []byte(""), 0644)
	cfg2 := &config.SSHConfig{KnownHostsFile: tmpFile}
	callback2, _ := createHostKeyCallback(cfg2, nil)
	if callback2 == nil {
		t.Error("expected a callback for existing file, got nil")
	}

	// Case 3: Empty path should try default (cannot easily mock home, but can check it doesn't panic)
	cfg3 := &config.SSHConfig{KnownHostsFile: ""}
	createHostKeyCallback(cfg3, nil)
}

func TestParseDiskSpace(t *testing.T) {
	// CheckDiskSpace uses c.ExecuteCommand which we can't easily mock here without refactor.
	// But we can test the internal logic if we isolate it.
}

func TestStrictHostKey(t *testing.T) {
	cfg := &config.SSHConfig{KnownHostsFile: filepath.Join(t.TempDir(), "missing"), StrictHostKey: true}
	if cb, err := createHostKeyCallback(cfg, nil); err == nil || cb != nil {
		t.Error("strict_host_key must refuse a missing known_hosts")
	}
}

func TestKnownHostKeyAlgorithms(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub, _ := ssh.NewPublicKey(&rsaKey.PublicKey)
	file := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(file, []byte(knownhosts.Line([]string{"[example.com]:2222"}, pub)+"\n"), 0644)
	cb, err := knownhosts.New(file)
	if err != nil {
		t.Fatal(err)
	}
	got := knownHostKeyAlgorithms(cb, "example.com:2222", 2222)
	if !slices.Contains(got, ssh.KeyAlgoRSASHA512) || slices.Contains(got, ssh.KeyAlgoED25519) {
		t.Errorf("algorithms for an RSA-only host: %v", got)
	}
	if got := knownHostKeyAlgorithms(cb, "other.com:22", 22); got != nil {
		t.Errorf("unknown host: %v", got)
	}
}
