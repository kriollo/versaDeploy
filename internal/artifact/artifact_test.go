package artifact

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/builder"
)

func TestGenerator_Compress(t *testing.T) {
	// Create temporary artifact directory
	artifactDir := t.TempDir()

	// Create some files and directories
	files := map[string]string{
		"index.php":           "<?php echo 'hello';",
		"public/style.css":    "body { color: red; }",
		"vendor/autoload.php": "<?php",
	}

	for path, content := range files {
		fullPath := filepath.Join(artifactDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0775)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Create generator
	g := NewGenerator(artifactDir, "20260127", "hash123")

	// Run compression
	archivePath := filepath.Join(t.TempDir(), "artifact.tar.gz")
	if err := g.Compress(archivePath); err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive file was not created")
	}

	// Verify content of archive
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	archivedFiles := make(map[string]bool)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		archivedFiles[header.Name] = true
	}

	expectedFiles := []string{
		"index.php",
		"public/style.css",
		"vendor/autoload.php",
		"public",
		"vendor",
	}

	for _, expected := range expectedFiles {
		if !archivedFiles[expected] {
			t.Errorf("expected file %s not found in archive", expected)
		}
	}
}

func TestGenerator_GenerateManifest(t *testing.T) {
	artifactDir := t.TempDir()
	g := NewGenerator(artifactDir, "1.0.0", "abc123")

	buildResult := &builder.BuildResult{
		PHPFilesChanged: 5,
		GoBinaryRebuilt: true,
	}

	if err := g.GenerateManifest(buildResult); err != nil {
		t.Fatalf("GenerateManifest() error = %v", err)
	}

	manifestPath := filepath.Join(artifactDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("manifest.json not created")
	}
}

func TestGenerator_Validate(t *testing.T) {
	artifactDir := t.TempDir()
	g := NewGenerator(artifactDir, "1.0.0", "abc123")

	// Should fail if manifest missing
	if err := g.Validate(); err == nil {
		t.Error("Validate() should fail when manifest is missing")
	}

	// Create manifest
	os.WriteFile(filepath.Join(artifactDir, "manifest.json"), []byte("{}"), 0644)

	// Now should pass
	if err := g.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestGenerateReleaseVersion(t *testing.T) {
	v := GenerateReleaseVersion()
	if len(v) != 15 { // YYYYMMDD-HHMMSS is 8 + 1 + 6 = 15
		t.Errorf("unexpected version format: %s", v)
	}
}
