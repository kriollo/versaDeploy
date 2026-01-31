package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

// BenchmarkBuild_Concurrent benchmarks concurrent builds
func BenchmarkBuild_Concurrent(b *testing.B) {
	repoDir := b.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(repoDir, "index.php"), []byte("<?php"), 0644)
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(repoDir, "app.js"), []byte("console.log('test')"), 0644)

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{
				Enabled:         true,
				ComposerCommand: "echo 'mock composer'",
			},
			Go: config.GoBuildConfig{
				Enabled:    false, // Disable to avoid actual compilation in benchmark
				TargetOS:   "linux",
				TargetArch: "amd64",
				BinaryName: "app",
			},
			Frontend: config.FrontendBuildConfig{
				Enabled:        true,
				NPMCommand:     "echo 'mock npm'",
				CompileCommand: "echo 'mock compile'",
			},
		},
	}

	cs := &changeset.ChangeSet{
		PHPFiles:        []string{"index.php"},
		GoFiles:         []string{"main.go"},
		FrontendFiles:   []string{"app.js"},
		ComposerChanged: true,
		PackageChanged:  true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		artifactDir := filepath.Join(b.TempDir(), "artifact")
		log, _ := logger.NewLogger("", false, false)
		builder := NewBuilder(repoDir, artifactDir, cfg, cs, log)
		_, err := builder.Build()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestBuild_ConcurrentCorrectness tests that concurrent builds produce correct results
func TestBuild_ConcurrentCorrectness(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(repoDir, "index.php"), []byte("<?php echo 'test';"), 0644)
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(repoDir, "app.js"), []byte("console.log('test')"), 0644)

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{
				Enabled:         true,
				ComposerCommand: "echo 'composer install'",
			},
			Go: config.GoBuildConfig{
				Enabled:    false, // Disable actual Go build
				TargetOS:   "linux",
				TargetArch: "amd64",
				BinaryName: "app",
			},
			Frontend: config.FrontendBuildConfig{
				Enabled:        true,
				NPMCommand:     "echo 'npm install'",
				CompileCommand: "echo 'npm run build'",
			},
		},
	}

	cs := &changeset.ChangeSet{
		PHPFiles:      []string{"index.php"},
		FrontendFiles: []string{"app.js"},
	}

	log, _ := logger.NewLogger("", false, false)
	builder := NewBuilder(repoDir, artifactDir, cfg, cs, log)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify build result
	if result.PHPFilesChanged != 1 {
		t.Errorf("Expected 1 PHP file changed, got %d", result.PHPFilesChanged)
	}
	if result.FrontendCompiled != 1 {
		t.Errorf("Expected 1 frontend file compiled, got %d", result.FrontendCompiled)
	}

	// Verify files were copied
	if _, err := os.Stat(filepath.Join(artifactDir, "app/index.php")); os.IsNotExist(err) {
		t.Error("index.php not found in artifact")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "app/app.js")); os.IsNotExist(err) {
		t.Error("app.js not found in artifact")
	}
}

// TestBuild_ConcurrentErrorPropagation tests that errors in concurrent builds are properly propagated
func TestBuild_ConcurrentErrorPropagation(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{
				Enabled:         true,
				ComposerCommand: "exit 1", // This will fail
			},
		},
	}

	cs := &changeset.ChangeSet{
		ComposerChanged: true,
	}

	log, _ := logger.NewLogger("", false, false)
	builder := NewBuilder(repoDir, artifactDir, cfg, cs, log)
	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error from failed build, got nil")
	}
}
