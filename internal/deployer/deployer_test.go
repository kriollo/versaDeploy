package deployer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

func TestNewDeployer(t *testing.T) {
	cfg := &config.Config{
		Project: "test-project",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
			},
		},
	}
	log, _ := logger.NewLogger("", false, false)

	// Valid environment
	d, err := NewDeployer(cfg, "prod", "repo/path", false, false, false, log)
	if err != nil {
		t.Fatalf("NewDeployer failed: %v", err)
	}
	if d.envName != "prod" {
		t.Errorf("expected prod, got %s", d.envName)
	}

	// Invalid environment
	_, err = NewDeployer(cfg, "staging", "repo/path", false, false, false, log)
	if err == nil {
		t.Error("expected error for invalid environment")
	}
}

func TestDeployer_ValidateLocalTools(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Builds: config.BuildsConfig{
					PHP: config.PHPBuildConfig{Enabled: false},
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, log)

	err := d.validateLocalTools()
	t.Logf("validateLocalTools returned: %v", err)
}

func TestDeployer_CalculateDirectorySize(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("123"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "sub"), 0775)
	os.WriteFile(filepath.Join(tmpDir, "sub/f2.txt"), []byte("45"), 0644)

	d := &Deployer{}
	size, err := d.calculateDirectorySize(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirectorySize failed: %v", err)
	}

	if size != 5 { // 3 + 2 bytes
		t.Errorf("expected 5, got %d", size)
	}
}

func TestDeployer_ValidateLocalTools_Go(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Builds: config.BuildsConfig{
					Go: config.GoBuildConfig{Enabled: true, TargetOS: "linux", TargetArch: "amd64", BinaryName: "app"},
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, log)
	err := d.validateLocalTools()
	// Should at least check for 'go'
	t.Logf("validateLocalTools (Go) returned: %v", err)
}

func TestDeployer_ValidateLocalTools_Frontend(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Builds: config.BuildsConfig{
					Frontend: config.FrontendBuildConfig{Enabled: true, CompileCommand: "npm run {file}"},
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, log)
	err := d.validateLocalTools()
	t.Logf("validateLocalTools (Frontend) returned: %v", err)
}
