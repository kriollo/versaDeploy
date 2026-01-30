package changeset

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkDetect_Sequential benchmarks the original sequential implementation
// This is for comparison purposes - the actual implementation is now concurrent
func BenchmarkDetect_Concurrent(b *testing.B) {
	// Create a temporary directory with many files
	tmpDir := b.TempDir()

	// Create 1000 test files
	for i := 0; i < 1000; i++ {
		filename := filepath.Join(tmpDir, "file"+string(rune(i))+".txt")
		os.WriteFile(filename, []byte("test content"), 0644)
	}

	detector := NewDetector(tmpDir, []string{}, []string{}, ".", ".", ".", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.Detect()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestDetect_Concurrent tests concurrent file hashing for correctness
func TestDetect_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"file1.php":  "<?php",
		"file2.go":   "package main",
		"file3.js":   "console.log('test')",
		"file4.twig": "{{ content }}",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	detector := NewDetector(tmpDir, []string{}, []string{}, ".", ".", ".", nil)
	cs, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() failed: %v", err)
	}

	// Verify all files were detected
	if len(cs.AllFileHashes) != 4 {
		t.Errorf("Expected 4 files, got %d", len(cs.AllFileHashes))
	}

	// Verify categorization
	if len(cs.PHPFiles) != 1 {
		t.Errorf("Expected 1 PHP file, got %d", len(cs.PHPFiles))
	}
	if len(cs.GoFiles) != 1 {
		t.Errorf("Expected 1 Go file, got %d", len(cs.GoFiles))
	}
	if len(cs.FrontendFiles) != 1 {
		t.Errorf("Expected 1 Frontend file, got %d", len(cs.FrontendFiles))
	}
	if len(cs.TwigFiles) != 1 {
		t.Errorf("Expected 1 Twig file, got %d", len(cs.TwigFiles))
	}
}

// TestDetect_ConcurrentLargeRepo tests with a larger repository
func TestDetect_ConcurrentLargeRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large repo test in short mode")
	}

	tmpDir := t.TempDir()

	// Create 500 files with valid filenames
	for i := 0; i < 500; i++ {
		filename := filepath.Join(tmpDir, "file_"+string(rune(i%26+'a'))+"_"+string(rune(i/26%26+'a'))+".php")
		if err := os.WriteFile(filename, []byte("<?php echo 'test';"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	detector := NewDetector(tmpDir, []string{}, []string{}, ".", ".", ".", nil)
	cs, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() failed: %v", err)
	}

	if len(cs.AllFileHashes) != 500 {
		t.Errorf("Expected 500 files, got %d", len(cs.AllFileHashes))
	}
}
