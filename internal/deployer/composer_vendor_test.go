package deployer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComposerVendorDir(t *testing.T) {
	dir := t.TempDir()
	if got := composerVendorDir(dir); got != "vendor" {
		t.Errorf("no composer.json: %q", got)
	}
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"config":{"vendor-dir":"versaWYS/vendor/"}}`), 0644)
	if got := composerVendorDir(dir); got != filepath.Clean("versaWYS/vendor") {
		t.Errorf("vendor-dir: %q", got)
	}
}
