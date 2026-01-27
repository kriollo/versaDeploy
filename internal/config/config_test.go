package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: Config{
				Project: "my-project",
				Environments: map[string]Environment{
					"prod": {
						SSH: SSHConfig{
							Host:    "example.com",
							User:    "deploy",
							KeyPath: "temp_key", // Will be created in test
						},
						RemotePath: "/var/www/app",
						Builds: BuildsConfig{
							PHP: PHPBuildConfig{Enabled: true},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing Remote Path",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH: SSHConfig{Host: "h", User: "u", KeyPath: "k"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid Remote Path",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "relative/path",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No Build Types Enabled",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds:     BuildsConfig{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Frontend Missing Placeholder",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Frontend: FrontendBuildConfig{
								Enabled:        true,
								CompileCommand: "no-placeholder",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temporary SSH key for valid cases
			if !tt.wantErr && tt.config.Environments["prod"].SSH.KeyPath != "" {
				keyPath := filepath.Join(t.TempDir(), "id_rsa")
				os.WriteFile(keyPath, []byte("fake-key"), 0600)

				// Update the config with the actual temp path
				env := tt.config.Environments["prod"]
				env.SSH.KeyPath = keyPath
				tt.config.Environments["prod"] = env
			}

			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	yamlContent := `
project: "test-app"
environments:
  staging:
    ssh:
      host: "staging.local"
      user: "admin"
      key_path: "${HOME}/.ssh/id_rsa"
    remote_path: "/tmp/app"
    builds:
      php:
        enabled: true
`
	tmpConfig := filepath.Join(t.TempDir(), "deploy.yml")
	os.WriteFile(tmpConfig, []byte(yamlContent), 0644)

	// setup fake home with forward slashes to avoid YAML escape issues
	home := filepath.ToSlash(t.TempDir())
	os.Setenv("HOME", home)
	keyPath := filepath.Join(home, ".ssh", "id_rsa")
	os.MkdirAll(filepath.Dir(keyPath), 0755)

	// On Windows, permissions are tricky. We'll use a very restricted mode
	// but the check in config.go might still fail if it's too strict for Windows.
	// For now, let's just write the file.
	os.WriteFile(keyPath, []byte("fake-key"), 0600)

	cfg, err := Load(tmpConfig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Project != "test-app" {
		t.Errorf("expected project test-app, got %s", cfg.Project)
	}

	env, _ := cfg.GetEnvironment("staging")
	if env.SSH.Host != "staging.local" {
		t.Errorf("expected host staging.local, got %s", env.SSH.Host)
	}

	// Verify new fields
	yamlContentWithNewFields := `
project: "new-test"
environments:
  prod:
    ssh:
      host: "prod.site"
      user: "root"
      key_path: "${HOME}/.ssh/id_rsa"
      use_ssh_agent: true
    remote_path: "/var/www"
    hook_timeout: 600
    builds:
      go:
        enabled: true
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "test-app"
`
	tmpConfig2 := filepath.Join(t.TempDir(), "deploy_new.yml")
	os.WriteFile(tmpConfig2, []byte(yamlContentWithNewFields), 0644)

	cfg2, err := Load(tmpConfig2)
	if err != nil {
		t.Fatalf("Load() with new fields error = %v", err)
	}

	prodEnv, _ := cfg2.GetEnvironment("prod")
	if prodEnv.SSH.UseSSHAgent != true {
		t.Error("expected UseSSHAgent to be true")
	}
	if prodEnv.HookTimeout != 600 {
		t.Errorf("expected HookTimeout 600, got %d", prodEnv.HookTimeout)
	}
}
