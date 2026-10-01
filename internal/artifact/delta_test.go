package artifact

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDelta(t *testing.T) {
	prev := map[string]string{
		"app/same.php":        "h1",
		"app/changed.php":     "h2",
		"app/deleted.php":     "h3",
		"app/vendor/old.php":  "h4",
		"app/vendor/keep.php": "h5",
	}
	cur := map[string]string{
		"app/same.php":        "h1",
		"app/changed.php":     "h2b",
		"app/new.php":         "h6",
		"app/vendor/keep.php": "h5",
	}
	upload, remove := Delta(prev, cur, []string{"app/vendor"})

	wantUpload := []string{"app/changed.php", "app/new.php", "app/vendor/keep.php"}
	wantRemove := []string{"app/changed.php", "app/deleted.php", "app/new.php", "app/vendor"}
	if !reflect.DeepEqual(upload, wantUpload) {
		t.Errorf("upload = %v, want %v", upload, wantUpload)
	}
	if !reflect.DeepEqual(remove, wantRemove) {
		t.Errorf("remove = %v, want %v", remove, wantRemove)
	}
}

func TestHashTreeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub dir"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "sub dir", "a b.txt"), []byte("x"), 0644)
	os.Symlink("sub dir", filepath.Join(dir, "app", "link"))

	hashes, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 2 || hashes["app/link"] != "link:sub dir" {
		t.Fatalf("unexpected hashes: %v", hashes)
	}
	if got := DecodeHashes(EncodeHashes(hashes)); !reflect.DeepEqual(got, hashes) {
		t.Errorf("round trip: %v != %v", got, hashes)
	}
}
