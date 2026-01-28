package builder

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
)

func TestBuilder_createArtifactStructure(t *testing.T) {
	artifactDir := t.TempDir()
	b := &Builder{
		artifactDir: artifactDir,
	}

	if err := b.createArtifactStructure(); err != nil {
		t.Fatalf("createArtifactStructure() error = %v", err)
	}

	dirs := []string{"app", "vendor", "node_modules", "public", "bin"}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(artifactDir, dir)); os.IsNotExist(err) {
			t.Errorf("directory %s was not created", dir)
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

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	os.MkdirAll(filepath.Join(src, "subdir"), 0755)
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(src, "subdir", "file2.txt"), []byte("2"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("file2.txt not copied")
	}
}

func TestNewBuilder(t *testing.T) {
	cfg := &config.Environment{}
	cs := &changeset.ChangeSet{}
	b := NewBuilder("repo", "artifact", cfg, cs)

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
	os.MkdirAll(filepath.Join(repoDir, "src"), 0755)
	os.WriteFile(filepath.Join(repoDir, "src/helpers.php"), []byte("<?php"), 0644)

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{Enabled: true},
		},
	}

	cs := &changeset.ChangeSet{
		PHPFiles: []string{"index.php", "src/helpers.php"},
	}

	b := NewBuilder(repoDir, artifactDir, cfg, cs)
	if err := b.createArtifactStructure(); err != nil {
		t.Fatal(err)
	}

	if err := b.buildPHP(); err != nil {
		t.Fatalf("buildPHP() error = %v", err)
	}

	if b.result.PHPFilesChanged != 2 {
		t.Errorf("expected 2 PHP files changed, got %d", b.result.PHPFilesChanged)
	}

	// Check if files exist in artifact
	if _, err := os.Stat(filepath.Join(artifactDir, "app/index.php")); os.IsNotExist(err) {
		t.Error("index.php not copied to artifact")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "app/src/helpers.php")); os.IsNotExist(err) {
		t.Error("src/helpers.php not copied to artifact")
	}
}

func TestBuilder_BuildPHP_TwigAndRoutes(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	os.WriteFile(filepath.Join(repoDir, "template.twig"), []byte("{{ template }}"), 0644)

	cfg := &config.Environment{
		Builds: config.BuildsConfig{
			PHP: config.PHPBuildConfig{Enabled: true},
		},
	}

	cs := &changeset.ChangeSet{
		TwigFiles:     []string{"template.twig"},
		RoutesChanged: true,
	}

	b := NewBuilder(repoDir, artifactDir, cfg, cs)
	b.createArtifactStructure()

	if err := b.buildPHP(); err != nil {
		t.Fatal(err)
	}

	if !b.result.TwigCacheCleanup {
		t.Error("expected TwigCacheCleanup to be true")
	}
	if !b.result.RouteCacheRegenerate {
		t.Error("expected RouteCacheRegenerate to be true")
	}

	if _, err := os.Stat(filepath.Join(artifactDir, "app/template.twig")); os.IsNotExist(err) {
		t.Error("template.twig not copied")
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
	b := NewBuilder(repoDir, artifactDir, cfg, cs)

	res, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if res.PHPFilesChanged != 0 || res.GoBinaryRebuilt != false {
		t.Error("expected no changes in result")
	}
}

func TestBuilder_Build_Fail(t *testing.T) {
	repoDir := t.TempDir()
	// Create a file where a directory should be
	artifactDir := filepath.Join(t.TempDir(), "blocked")
	os.WriteFile(artifactDir, []byte("blocked"), 0644)

	b := NewBuilder(repoDir, artifactDir, &config.Environment{}, &changeset.ChangeSet{})
	_, err := b.Build()
	if err == nil {
		t.Error("expected error when artifact structure cannot be created")
	}
}
func TestBuilder_Build_Subdirectories(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	// Create structure: api/composer.json
	os.MkdirAll(filepath.Join(repoDir, "api"), 0755)
	os.WriteFile(filepath.Join(repoDir, "api/composer.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(repoDir, "api/index.php"), []byte("<?php"), 0644)

	var mockCmd string
	if runtime.GOOS == "windows" {
		// Commands are run in ProjectRoot, so we use relative paths
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

	b := NewBuilder(repoDir, artifactDir, cfg, cs)
	_, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Verify vendor copied from subdirectory
	if _, err := os.Stat(filepath.Join(artifactDir, "vendor/autoload.php")); os.IsNotExist(err) {
		t.Error("vendor/autoload.php not copied from subdirectory")
	}

	// Verify PHP file copied
	if _, err := os.Stat(filepath.Join(artifactDir, "app/api/index.php")); os.IsNotExist(err) {
		t.Error("api/index.php not copied to artifact")
	}
}

func TestBuilder_CopyOtherFiles(t *testing.T) {
	repoDir := t.TempDir()
	artifactDir := t.TempDir()

	os.MkdirAll(filepath.Join(repoDir, "public/images"), 0755)
	os.WriteFile(filepath.Join(repoDir, "public/images/logo.png"), []byte("png"), 0644)

	cs := &changeset.ChangeSet{
		OtherFiles: []string{"public/images/logo.png"},
	}

	b := NewBuilder(repoDir, artifactDir, &config.Environment{}, cs)
	b.createArtifactStructure()

	err := b.copyOtherFiles()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(artifactDir, "app/public/images/logo.png")); os.IsNotExist(err) {
		t.Error("logo.png not copied to artifact")
	}
}
