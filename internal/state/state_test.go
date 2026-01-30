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

func TestParse_Errors(t *testing.T) {
	// Empty data
	_, err := Parse([]byte(""))
	if err == nil {
		t.Error("expected error for empty data")
	}

	// Invalid JSON
	_, err = Parse([]byte("{invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	// Wrong version
	badVersion := `{"version": "0.1", "last_deploy": {}}`
	_, err = Parse([]byte(badVersion))
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestSortReleases(t *testing.T) {
	releases := []string{
		"20260129-100000",
		"20260130-100000",
		"20260130-090000",
		"20260128-100000",
	}

	SortReleases(releases)

	expected := []string{
		"20260130-100000",
		"20260130-090000",
		"20260129-100000",
		"20260128-100000",
	}

	for i := range expected {
		if releases[i] != expected[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expected[i], releases[i])
		}
	}
}
