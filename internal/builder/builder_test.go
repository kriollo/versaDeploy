package builder

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

func TestBuilder_copyEntireRepo(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create some files in repo
	os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("1"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "dir1"), 0775)
	os.WriteFile(filepath.Join(repoDir, "dir1/file2.txt"), []byte("2"), 0644)

	b := &Builder{
		repoPath:    repoDir,
		artifactDir: artifactDir,
	}

	if err := b.copyEntireRepo(); err != nil {
		t.Fatalf("copyEntireRepo() error = %v", err)
	}

	// Everything should be inside artifactDir/app
	dirs := []string{"app", "app/dir1"}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(artifactDir, dir)); os.IsNotExist(err) {
			t.Errorf("directory %s was not created", dir)
		}
	}

	files := []struct {
		path    string
		content string
	}{
		{"app/file1.txt", "1"},
		{"app/dir1/file2.txt", "2"},
	}

	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(artifactDir, f.path))
		if err != nil {
			t.Errorf("failed to read file %s: %v", f.path, err)
			continue
		}
		if string(content) != f.content {
			t.Errorf("expected %s, got %s", f.content, string(content))
		}
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := []byte("test content")
	os.WriteFile(src, content, 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != string(content) {
		t.Errorf("expected %s, got %s", string(content), string(got))
	}
}

func TestNewBuilder(t *testing.T) {
	cfg := &config.Environment{}
	cs := &changeset.ChangeSet{}
	log, _ := logger.NewLogger("", false, false)
	b := NewBuilder("repo", "artifact", cfg, cs, log)

	if b.repoPath != "repo" || b.artifactDir != "artifact" {
		t.Error("NewBuilder fields not correctly initialized")
	}
	if b.result == nil {
		t.Error("result should be initialized")
	}
}

func TestBuilder_BuildPHP_NoComposer(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create some files in repo
	os.WriteFile(filepath.Join(repoDir, "index.php"), []byte("<?php"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "src"), 0775)
	os.WriteFile(filepath.Join(repoDir, "src/helpers.php"), []byte("<?php"), 0644)

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{Enabled: true},
		},
	}

	cs := &changeset.ChangeSet{
		PHPFiles: []string{"index.php", "src/helpers.php"},
	}

	log, _ := logger.NewLogger("", false, false)
	b := NewBuilder(repoDir, artifactDir, cfg, cs, log)
	if err := b.copyEntireRepo(); err != nil {
		t.Fatal(err)
	}

	if err := b.buildPHP(); err != nil {
		t.Fatalf("buildPHP() error = %v", err)
	}

	if b.result.PHPFilesChanged != 2 {
		t.Errorf("expected 2 PHP files changed, got %d", b.result.PHPFilesChanged)
	}

	// Check if files exist in artifact (all should be under app/)
	if _, err := os.Stat(filepath.Join(artifactDir, "app/index.php")); os.IsNotExist(err) {
		t.Error("index.php not found in artifact/app")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "app/src/helpers.php")); os.IsNotExist(err) {
		t.Error("src/helpers.php not found in artifact/app")
	}
}

func TestBuilder_CleanupIgnoredPaths(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create structure
	os.MkdirAll(filepath.Join(repoDir, "src"), 0775)
	os.WriteFile(filepath.Join(repoDir, "src/main.go"), []byte("go"), 0644)
	os.WriteFile(filepath.Join(repoDir, "keep.txt"), []byte("keep"), 0644)

	cfg := &config.Environment{
		Ignored: []string{"src"},
	}

	log, _ := logger.NewLogger("", false, false)
	b := NewBuilder(repoDir, artifactDir, cfg, &changeset.ChangeSet{}, log)

	// Step 1: Copy everything
	if err := b.copyEntireRepo(); err != nil {
		t.Fatal(err)
	}

	// Verify src exists before cleanup
	if _, err := os.Stat(filepath.Join(artifactDir, "app/src")); os.IsNotExist(err) {
		t.Fatal("src should be copied initially")
	}

	// Step 2: Cleanup
	if err := b.cleanupIgnoredPaths(); err != nil {
		t.Fatal(err)
	}

	// Verify src is gone but keep.txt remains
	if _, err := os.Stat(filepath.Join(artifactDir, "app/src")); !os.IsNotExist(err) {
		t.Error("src should have been removed by cleanup")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "app/keep.txt")); os.IsNotExist(err) {
		t.Error("keep.txt should have been preserved")
	}
}

func TestBuilder_Build_DisabledComponents(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP:      config.PHPBuildConfig{Enabled: false},
			Go:       config.GoBuildConfig{Enabled: false},
			Frontend: config.FrontendBuildConfig{Enabled: false},
		},
	}

	cs := &changeset.ChangeSet{}
	log, _ := logger.NewLogger("", false, false)
	b := NewBuilder(repoDir, artifactDir, cfg, cs, log)

	res, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if res.PHPFilesChanged != 0 || res.GoBinaryRebuilt != false {
		t.Error("expected no changes in result")
	}
}

func TestBuilder_Build_Subdirectories(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create structure: api/composer.json
	os.MkdirAll(filepath.Join(repoDir, "api"), 0775)
	os.WriteFile(filepath.Join(repoDir, "api/composer.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(repoDir, "api/index.php"), []byte("<?php"), 0644)

	var mockCmd string
	if runtime.GOOS == "windows" {
		// Commands are run in ProjectRoot (relative to app/), so we use relative paths
		mockCmd = "mkdir vendor && echo . > vendor\\autoload.php"
	} else {
		mockCmd = "mkdir -p vendor && touch vendor/autoload.php"
	}

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{
				Enabled:     true,
				ProjectRoot: "api",
				// Mock command that creates vendor
				ComposerCommand: mockCmd,
			},
		},
	}

	cs := &changeset.ChangeSet{
		ComposerChanged: true,
		PHPFiles:        []string{"api/index.php"},
	}

	log, _ := logger.NewLogger("", false, false)
	b := NewBuilder(repoDir, artifactDir, cfg, cs, log)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Verify vendor created in subdirectory (which is inside app/)
	if _, err := os.Stat(filepath.Join(artifactDir, "app/api/vendor/autoload.php")); os.IsNotExist(err) {
		t.Error("vendor/autoload.php not found in api/vendor")
	}

	// Verify PHP file copied
	if _, err := os.Stat(filepath.Join(artifactDir, "app/api/index.php")); os.IsNotExist(err) {
		t.Error("api/index.php not found in artifact/app/api")
	}
}
