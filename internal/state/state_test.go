package state

import (
	"testing"
)

func TestDeployLock_ToJSON_And_Parse(t *testing.T) {
	hashes := map[string]string{
		"main.go": "hash1",
	}
	lock := New("abc123", "20260127-120000", hashes, "chash", "phash", "ghash")

	data, err := lock.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if parsed.Version != LockFileVersion {
		t.Errorf("expected version %s, got %s", LockFileVersion, parsed.Version)
	}

	if parsed.LastDeploy.CommitHash != "abc123" {
		t.Errorf("expected commit abc123, got %s", parsed.LastDeploy.CommitHash)
	}

	if h, ok := parsed.GetFileHash("main.go"); !ok || h != "hash1" {
		t.Errorf("expected hash1 for main.go, got %s (exists=%v)", h, ok)
	}
}

func TestIsFirstDeploy(t *testing.T) {
	if !IsFirstDeploy(nil) {
		t.Error("nil lock should be first deploy")
	}

	lock := &DeployLock{}
	if !IsFirstDeploy(lock) {
		t.Error("empty lock should be first deploy")
	}

	lock.LastDeploy.FileHashes = map[string]string{"f": "h"}
	if IsFirstDeploy(lock) {
		t.Error("lock with hashes should NOT be first deploy")
	}
}
