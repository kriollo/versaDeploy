package deployer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	d, err := NewDeployer(cfg, "prod", "repo/path", false, false, false, false, log)
	if err != nil {
		t.Fatalf("NewDeployer failed: %v", err)
	}
	if d.envName != "prod" {
		t.Errorf("expected prod, got %s", d.envName)
	}

	// Invalid environment
	_, err = NewDeployer(cfg, "staging", "repo/path", false, false, false, false, log)
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

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)

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

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
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

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.validateLocalTools()
	t.Logf("validateLocalTools (Frontend) returned: %v", err)
}
func TestDeployer_SkipDirtyCheck(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
			},
		},
	}

	// Case 1: skipDirtyCheck = false (default)
	d1, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	if d1.skipDirtyCheck {
		t.Error("expected skipDirtyCheck to be false by default")
	}

	// Case 2: skipDirtyCheck = true
	d2, _ := NewDeployer(cfg, "prod", ".", false, false, false, true, log)
	if !d2.skipDirtyCheck {
		t.Error("expected skipDirtyCheck to be true when requested")
	}
}

func TestDeployer_SendNotification_Success(t *testing.T) {
	var received map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test-project",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Notifications: config.NotificationConfig{
					WebhookURL: ts.URL,
					OnSuccess:  true,
					OnFailure:  true,
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)

	// Test success notification
	d.sendNotification("20260326-120000", "abc123", nil, 30*time.Second)

	if received == nil {
		t.Fatal("expected notification to be sent")
	}
	if received["status"] != "success" {
		t.Errorf("expected status=success, got %v", received["status"])
	}
	if received["project"] != "test-project" {
		t.Errorf("expected project=test-project, got %v", received["project"])
	}
	if received["release"] != "20260326-120000" {
		t.Errorf("expected release=20260326-120000, got %v", received["release"])
	}
}

func TestDeployer_SendNotification_Failure(t *testing.T) {
	var received map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test-project",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Notifications: config.NotificationConfig{
					WebhookURL: ts.URL,
					OnSuccess:  true,
					OnFailure:  true,
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	d.sendNotification("20260326-120000", "abc123", fmt.Errorf("build failed"), 10*time.Second)

	if received == nil {
		t.Fatal("expected failure notification to be sent")
	}
	if received["status"] != "failure" {
		t.Errorf("expected status=failure, got %v", received["status"])
	}
	if received["error"] != "build failed" {
		t.Errorf("expected error='build failed', got %v", received["error"])
	}
}

func TestDeployer_SendNotification_NoWebhook(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {RemotePath: "/var/www"},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	// Should not panic when no webhook is configured
	d.sendNotification("v1", "abc", nil, time.Second)
}

func TestDeployer_SendNotification_OnSuccessDisabled(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer ts.Close()

	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				Notifications: config.NotificationConfig{
					WebhookURL: ts.URL,
					OnSuccess:  false,
					OnFailure:  true,
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	d.sendNotification("v1", "abc", nil, time.Second)

	if called {
		t.Error("notification should NOT be sent when on_success=false and deploy succeeded")
	}
}

func TestDeployer_PerformHealthCheck_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				HealthCheck: config.HealthCheckConfig{
					URL:            ts.URL,
					ExpectedStatus: 200,
					Timeout:        5,
					Retries:        2,
					RetryDelay:     1,
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.performHealthCheck(nil, nil)
	if err != nil {
		t.Fatalf("health check should pass: %v", err)
	}
}

func TestDeployer_PerformHealthCheck_WrongStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				HealthCheck: config.HealthCheckConfig{
					URL:            ts.URL,
					ExpectedStatus: 200,
					Timeout:        2,
					Retries:        2,
					RetryDelay:     1,
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.performHealthCheck(nil, nil)
	if err == nil {
		t.Fatal("health check should fail with wrong status code")
	}
}

func TestDeployer_PerformHealthCheck_NoURL(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath:  "/var/www",
				HealthCheck: config.HealthCheckConfig{},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.performHealthCheck(nil, nil)
	if err != nil {
		t.Fatalf("health check with no URL should be a no-op: %v", err)
	}
}

func TestDeployer_ExecuteServicesReload_NoCommands(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {RemotePath: "/var/www"},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	// Should not panic when no services or SSH client
	d.executeServicesReload(nil)
}

func TestDeployer_RunHooks_NoSSH(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				SSH: config.SSHConfig{
					Host:    "invalid-host-that-does-not-exist.local",
					User:    "testuser",
					KeyPath: "/nonexistent/key",
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.RunHooks(nil)
	if err == nil {
		t.Error("RunHooks should fail when SSH connection fails")
	}
}

func TestDeployer_RollbackTo_NoSSH(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				SSH: config.SSHConfig{
					Host:    "invalid-host-that-does-not-exist.local",
					User:    "testuser",
					KeyPath: "/nonexistent/key",
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	err := d.RollbackTo("20260326-120000")
	if err == nil {
		t.Error("RollbackTo should fail when SSH connection fails")
	}
}

func TestDeployer_ExecRemoteCommand_NoSSH(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{
		Project: "test",
		Environments: map[string]config.Environment{
			"prod": {
				RemotePath: "/var/www",
				SSH: config.SSHConfig{
					Host:    "invalid-host-that-does-not-exist.local",
					User:    "testuser",
					KeyPath: "/nonexistent/key",
				},
			},
		},
	}

	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	_, err := d.ExecRemoteCommand("ls -la")
	if err == nil {
		t.Error("ExecRemoteCommand should fail when SSH connection fails")
	}
}
