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
			name: "Missing Project Name",
			config: Config{
				Environments: map[string]Environment{
					"prod": {},
				},
			},
			wantErr: true,
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
		{
			name: "Go Missing target_os",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Go: GoBuildConfig{Enabled: true},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Go Missing target_arch",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Go: GoBuildConfig{
								Enabled:  true,
								TargetOS: "linux",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Go Missing BinaryName",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Go: GoBuildConfig{
								Enabled:    true,
								TargetOS:   "linux",
								TargetArch: "amd64",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Frontend Missing NPMCommand (has default)",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Frontend: FrontendBuildConfig{
								Enabled:        true,
								CompileCommand: "npm run {file}",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "SSH Key Not Found",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "non-existent-key"},
						RemotePath: "/var/www",
						Builds:     BuildsConfig{PHP: PHPBuildConfig{Enabled: true}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Missing SSH Host",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Missing SSH User",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", KeyPath: "k"},
						RemotePath: "/var/www",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Frontend Missing CompileCommand",
			config: Config{
				Project: "test",
				Environments: map[string]Environment{
					"prod": {
						SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath: "/var/www",
						Builds: BuildsConfig{
							Frontend: FrontendBuildConfig{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Config with Advanced Fields",
			config: Config{
				Project: "advanced-test",
				Environments: map[string]Environment{
					"prod": {
						SSH:            SSHConfig{Host: "h", User: "u", KeyPath: "k"},
						RemotePath:     "/var/www",
						SharedPaths:    []string{"logs", "uploads"},
						PreservedPaths: []string{".env"},
						Builds: BuildsConfig{
							PHP: PHPBuildConfig{
								Enabled:       true,
								ReusablePaths: []string{"vendor", "custom"},
							},
							Frontend: FrontendBuildConfig{
								Enabled:        true,
								CompileCommand: "npm run build -- {file}",
								ReusablePaths:  []string{"dist", "node_modules"},
							},
						},
					},
				},
			},
			wantErr: false,
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

func TestLoad_Fail(t *testing.T) {
	_, err := Load("non-existent.yml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpConfig := filepath.Join(t.TempDir(), "invalid.yml")
	os.WriteFile(tmpConfig, []byte("invalid: yaml: :"), 0644)

	_, err := Load(tmpConfig)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfig_Validate_MultipleEnvs(t *testing.T) {
	cfg := Config{
		Project: "test",
		Environments: map[string]Environment{
			"prod": {
				SSH:        SSHConfig{Host: "h", User: "u", KeyPath: "k"},
				RemotePath: "/var/www",
				Builds:     BuildsConfig{PHP: PHPBuildConfig{Enabled: true}},
			},
			"staging": {
				SSH:        SSHConfig{Host: "h2", User: "u2", KeyPath: "k2"},
				RemotePath: "/var/www/staging",
				Builds:     BuildsConfig{PHP: PHPBuildConfig{Enabled: true}},
			},
		},
	}

	// Create keys
	tmpDir := t.TempDir()
	k1 := filepath.Join(tmpDir, "k1")
	k2 := filepath.Join(tmpDir, "k2")
	os.WriteFile(k1, []byte("f"), 0600)
	os.WriteFile(k2, []byte("f"), 0600)

	e1 := cfg.Environments["prod"]
	e1.SSH.KeyPath = k1
	cfg.Environments["prod"] = e1

	e2 := cfg.Environments["staging"]
	e2.SSH.KeyPath = k2
	cfg.Environments["staging"] = e2

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed for multi-env: %v", err)
	}
}

func TestConfig_Validate_NoEnvs(t *testing.T) {
	cfg := Config{Project: "test"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for no environments")
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

func TestConfig_GetEnvironment(t *testing.T) {
	cfg := Config{
		Environments: map[string]Environment{
			"prod": {RemotePath: "/var/www"},
		},
	}

	env, err := cfg.GetEnvironment("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.RemotePath != "/var/www" {
		t.Errorf("expected /var/www, got %s", env.RemotePath)
	}

	_, err = cfg.GetEnvironment("non-existent")
	if err == nil {
		t.Error("expected error for non-existent environment")
	}
}

func TestInterpolateEnvVars(t *testing.T) {
	os.Setenv("VAR1", "val1")
	os.Setenv("VAR2", "val2")
	defer os.Unsetenv("VAR1")
	defer os.Unsetenv("VAR2")

	tests := []struct {
		input    string
		expected string
	}{
		{"${VAR1}", "val1"},
		{"$VAR1", "val1"},
		{"${VAR1}-${VAR2}", "val1-val2"},
		{"no-vars", "no-vars"},
		{"${MISSING}", ""},
	}

	for _, tt := range tests {
		got := interpolateEnvVars(tt.input)
		if got != tt.expected {
			t.Errorf("interpolateEnvVars(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestConfig_Validate_Defaults(t *testing.T) {
	cfg := Config{
		Project: "test",
		Environments: map[string]Environment{
			"prod": {
				SSH: SSHConfig{
					Host:    "host",
					User:    "user",
					KeyPath: "temp_key",
				},
				RemotePath: "/var/www",
				Builds: BuildsConfig{
					PHP: PHPBuildConfig{Enabled: true},
				},
			},
		},
	}

	// Create temp key
	keyPath := filepath.Join(t.TempDir(), "id_rsa")
	os.WriteFile(keyPath, []byte("fake"), 0600)
	env := cfg.Environments["prod"]
	env.SSH.KeyPath = keyPath
	cfg.Environments["prod"] = env

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	updatedEnv := cfg.Environments["prod"]
	if updatedEnv.SSH.Port != 22 {
		t.Errorf("expected default port 22, got %d", updatedEnv.SSH.Port)
	}
	if updatedEnv.Builds.PHP.ComposerCommand == "" {
		t.Error("expected default composer command to be set")
	}
	if len(updatedEnv.Ignored) == 0 {
		t.Error("expected default ignored paths to be set")
	}
}
