package changeset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/state"
)

func TestHashFile(t *testing.T) {
	// Create a temporary file
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate hash
	hash, err := hashFile(tmpFile)
	if err != nil {
		t.Fatalf("hashFile failed: %v", err)
	}

	// Expected SHA256 for "hello world"
	expected := "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("expected hash %s, got %s", expected, hash)
	}
}

func TestDetector_ShouldIgnore(t *testing.T) {
	ignored := []string{".git", "vendor", "node_modules/cache"}
	d := NewDetector("", ignored, nil, "", "", "", nil)

	tests := []struct {
		path   string
		expect bool
	}{
		{".git/config", true},
		{"vendor/autoload.php", true},
		{"node_modules/cache/file.js", true},
		{"app/Controller.php", false},
		{"public/index.php", false},
	}

	for _, tt := range tests {
		if got := d.shouldIgnore(tt.path); got != tt.expect {
			t.Errorf("shouldIgnore(%s) = %v; want %v", tt.path, got, tt.expect)
		}
	}
}

func TestDetector_Detect(t *testing.T) {
	repoDir := t.TempDir()

	// Create some files
	files := map[string]string{
		"app/main.php":     "<?php echo 'hello';",
		"go.mod":           "module test",
		"main.go":          "package main",
		"package.json":     "{}",
		"public/app.js":    "console.log('hi');",
		".git/config":      "hidden",
		"ignored/file.txt": "ignored",
	}

	for path, content := range files {
		fullPath := filepath.Join(repoDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	ignored := []string{".git", "ignored"}
	routes := []string{"app/routes.php"}

	// Test first deployment (no previous lock)
	detector := NewDetector(repoDir, ignored, routes, "", "", "", nil)
	cs, err := detector.Detect()
	if err != nil {
		t.Fatal(err)
	}

	if !cs.HasChanges() {
		t.Error("expected changes for initial deploy")
	}

	if len(cs.PHPFiles) != 1 || cs.PHPFiles[0] != "app/main.php" {
		t.Errorf("expected 1 PHP file app/main.php, got %v", cs.PHPFiles)
	}

	if !cs.GoModChanged {
		t.Error("expected go.mod changed")
	}

	// Test second deployment with some changes
	previousLock := &state.DeployLock{
		LastDeploy: state.DeployInfo{
			FileHashes: map[string]string{
				"app/main.php":  cs.AllFileHashes["app/main.php"],
				"main.go":       cs.AllFileHashes["main.go"],
				"package.json":  cs.AllFileHashes["package.json"],
				"public/app.js": "old-hash", // Changed
			},
			GoModHash: cs.GoModHash, // Not changed
		},
	}

	detector = NewDetector(repoDir, ignored, routes, "", "", "", previousLock)
	cs2, err := detector.Detect()
	if err != nil {
		t.Fatal(err)
	}

	if len(cs2.PHPFiles) != 0 {
		t.Errorf("expected 0 PHP files to change, got %v", cs2.PHPFiles)
	}

	if len(cs2.FrontendFiles) != 1 || cs2.FrontendFiles[0] != "public/app.js" {
		t.Errorf("expected 1 frontend file public/app.js to change, got %v", cs2.FrontendFiles)
	}

	if cs2.GoModChanged {
		t.Error("expected go.mod NOT changed")
	}

	// Test case for Composer and routes
	os.WriteFile(filepath.Join(repoDir, "composer.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(repoDir, "app/routes.php"), []byte("<?php"), 0644)
	os.WriteFile(filepath.Join(repoDir, "app/view.twig"), []byte("twig"), 0644)

	detector = NewDetector(repoDir, ignored, routes, "", "", "", cs2.AllFileHashesAsLock()) // Use previous hashes
	cs3, err := detector.Detect()
	if err != nil {
		t.Fatal(err)
	}

	if !cs3.ComposerChanged {
		t.Error("expected composer.json changed")
	}
	if !cs3.RoutesChanged {
		t.Error("expected routes changed")
	}
	if len(cs3.TwigFiles) != 1 || cs3.TwigFiles[0] != "app/view.twig" {
		t.Error("expected twig file changed")
	}
}

func TestDetector_Detect_IgnoredButCritical(t *testing.T) {
	repoDir := t.TempDir()

	// Create a .vue file in an ignored directory
	vuePath := filepath.Join(repoDir, "src/components/App.vue")
	os.MkdirAll(filepath.Dir(vuePath), 0755)
	os.WriteFile(vuePath, []byte("<template>old</template>"), 0644)

	ignored := []string{"src"} // Entire src folder is ignored

	// First deploy
	d1 := NewDetector(repoDir, ignored, nil, "", "", "", nil)
	cs1, _ := d1.Detect()

	// Modify the .vue file
	os.WriteFile(vuePath, []byte("<template>new</template>"), 0644)

	// Detect changes for second deploy
	d2 := NewDetector(repoDir, ignored, nil, "", "", "", cs1.AllFileHashesAsLock())
	cs2, _ := d2.Detect()

	if len(cs2.FrontendFiles) != 1 || cs2.FrontendFiles[0] != "src/components/App.vue" {
		t.Errorf("expected 1 Frontend file to change despite being in ignored 'src', got %v", cs2.FrontendFiles)
	}
}

func (cs *ChangeSet) AllFileHashesAsLock() *state.DeployLock {
	return &state.DeployLock{
		LastDeploy: state.DeployInfo{
			FileHashes:      cs.AllFileHashes,
			GoModHash:       cs.GoModHash,
			ComposerHash:    cs.ComposerHash,
			PackageJSONHash: cs.PackageHash,
		},
	}
}

func TestHashFile_Fail(t *testing.T) {
	_, err := hashFile("non-existent")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestDetector_Detect_Fail(t *testing.T) {
	d := NewDetector("/non/existent/path", nil, nil, "", "", "", nil)
	_, err := d.Detect()
	if err == nil {
		t.Error("expected error for non-existent repo path")
	}
}
