package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	verserrors "github.com/user/versaDeploy/internal/errors"
	"gopkg.in/yaml.v3"
)

// Config represents the deploy.yml structure
type Config struct {
	Project      string                 `yaml:"project"`
	Environments map[string]Environment `yaml:"environments"`
}

// Environment represents a single deployment environment
type Environment struct {
	SSH         SSHConfig    `yaml:"ssh"`
	RemotePath  string       `yaml:"remote_path"`
	Builds      BuildsConfig `yaml:"builds"`
	PostDeploy  []string     `yaml:"post_deploy"`
	Ignored     []string     `yaml:"ignored_paths"`
	SharedPaths []string     `yaml:"shared_paths"` // Paths to persist between releases (e.g. storage, uploads)
	RouteFiles  []string     `yaml:"route_files"`  // Files that trigger route cache regeneration
	HookTimeout int          `yaml:"hook_timeout"` // Timeout for post-deploy hooks in seconds
}

// SSHConfig holds SSH connection details
type SSHConfig struct {
	Host           string `yaml:"host"`
	User           string `yaml:"user"`
	KeyPath        string `yaml:"key_path"`
	Port           int    `yaml:"port"`             // Default: 22
	KnownHostsFile string `yaml:"known_hosts_file"` // Optional: path to known_hosts file
	UseSSHAgent    bool   `yaml:"use_ssh_agent"`    // Optional: use SSH agent for authentication
}

// BuildsConfig holds build configuration for each language
type BuildsConfig struct {
	PHP      PHPBuildConfig      `yaml:"php"`
	Go       GoBuildConfig       `yaml:"go"`
	Frontend FrontendBuildConfig `yaml:"frontend"`
}

// PHPBuildConfig holds PHP build settings
type PHPBuildConfig struct {
	Enabled         bool   `yaml:"enabled"`
	ProjectRoot     string `yaml:"root"` // Subdirectory for composer.json
	ComposerCommand string `yaml:"composer_command"`
}

// GoBuildConfig holds Go build settings
type GoBuildConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ProjectRoot string `yaml:"root"` // Subdirectory for go.mod
	TargetOS    string `yaml:"target_os"`
	TargetArch  string `yaml:"target_arch"`
	BinaryName  string `yaml:"binary_name"`
	BuildFlags  string `yaml:"build_flags"` // Optional additional flags
}

// FrontendBuildConfig holds frontend build settings
type FrontendBuildConfig struct {
	Enabled           bool   `yaml:"enabled"`
	ProjectRoot       string `yaml:"root"`            // Subdirectory for package.json
	CompileCommand    string `yaml:"compile_command"` // {file} placeholder
	NPMCommand        string `yaml:"npm_command"`
	CleanupDevDeps    bool   `yaml:"cleanup_dev_deps"`   // Remove dev deps after build
	ProductionCommand string `yaml:"production_command"` // Command for production-only install
}

// Load reads and parses deploy.yml
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Interpolate environment variables
	content := interpolateEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate performs validation on the configuration
func (c *Config) Validate() error {
	if c.Project == "" {
		return verserrors.New(verserrors.CodeConfigInvalid, "Project name is missing in config", "Add 'project: \"your-project-name\"' at the top of your deploy.yml", nil)
	}

	if len(c.Environments) == 0 {
		return fmt.Errorf("at least one environment must be defined")
	}

	for envName := range c.Environments {
		env := c.Environments[envName]
		if err := env.Validate(envName); err != nil {
			return err
		}
		c.Environments[envName] = env
	}

	return nil
}

// Validate validates a single environment configuration
func (e *Environment) Validate(envName string) error {
	// SSH validation
	if e.SSH.Host == "" {
		return fmt.Errorf("environment %s: ssh.host is required", envName)
	}
	if e.SSH.User == "" {
		return fmt.Errorf("environment %s: ssh.user is required", envName)
	}
	if e.SSH.KeyPath == "" {
		return fmt.Errorf("environment %s: ssh.key_path is required", envName)
	}

	// Expand home directory in key path
	if strings.HasPrefix(e.SSH.KeyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("environment %s: failed to expand home directory: %w", envName, err)
		}
		e.SSH.KeyPath = filepath.Join(home, e.SSH.KeyPath[2:])
	}

	// Validate SSH key exists
	if _, err := os.Stat(e.SSH.KeyPath); os.IsNotExist(err) {
		return fmt.Errorf("environment %s: ssh key not found: %s", envName, e.SSH.KeyPath)
	}

	// Validate SSH key permissions (should be 0600 or stricter)
	info, err := os.Stat(e.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("environment %s: failed to stat ssh key: %w", envName, err)
	}
	mode := info.Mode().Perm()
	if runtime.GOOS != "windows" && mode&0077 != 0 {
		return verserrors.New(verserrors.CodeConfigInvalid, fmt.Sprintf("Environment %s: SSH key has insecure permissions (%o)", envName, mode), "Run 'chmod 600 "+e.SSH.KeyPath+"' to fix this.", nil)
	}

	// Default SSH port
	if e.SSH.Port == 0 {
		e.SSH.Port = 22
	}

	// Remote path validation
	if e.RemotePath == "" {
		return verserrors.New(verserrors.CodeConfigInvalid, fmt.Sprintf("Environment %s: remote_path is required", envName), "Add 'remote_path: \"/path/to/app\"' to your configuration.", nil)
	}

	if !strings.HasPrefix(e.RemotePath, "/") && !strings.Contains(e.RemotePath, ":") {
		// Very basic check for absolute path (Unix or Windows-style remote)
		return verserrors.New(verserrors.CodeConfigInvalid, fmt.Sprintf("Environment %s: remote_path must be an absolute path", envName), "Ensure 'remote_path' starts with / (for Linux) or a drive letter (for Windows).", nil)
	}

	// At least one build type must be enabled
	if !e.Builds.PHP.Enabled && !e.Builds.Go.Enabled && !e.Builds.Frontend.Enabled {
		return fmt.Errorf("environment %s: at least one build type must be enabled", envName)
	}

	// Validate PHP config
	if e.Builds.PHP.Enabled {
		if e.Builds.PHP.ComposerCommand == "" {
			e.Builds.PHP.ComposerCommand = "composer install --no-dev --optimize-autoloader --classmap-authoritative"
		}
	}

	// Validate Go config
	if e.Builds.Go.Enabled {
		if e.Builds.Go.TargetOS == "" {
			return fmt.Errorf("environment %s: go.target_os is required when go builds are enabled", envName)
		}
		if e.Builds.Go.TargetArch == "" {
			return fmt.Errorf("environment %s: go.target_arch is required when go builds are enabled", envName)
		}
		if e.Builds.Go.BinaryName == "" {
			return fmt.Errorf("environment %s: go.binary_name is required when go builds are enabled", envName)
		}
	}

	// Validate Frontend config
	if e.Builds.Frontend.Enabled {
		if e.Builds.Frontend.CompileCommand == "" {
			return fmt.Errorf("environment %s: frontend.compile_command is required when frontend builds are enabled", envName)
		}
		if e.Builds.Frontend.NPMCommand == "" {
			e.Builds.Frontend.NPMCommand = "npm ci --only=production"
		}
		// Set default production command if cleanup is enabled
		if e.Builds.Frontend.CleanupDevDeps && e.Builds.Frontend.ProductionCommand == "" {
			e.Builds.Frontend.ProductionCommand = "pnpm install --production"
		}
	}

	// Default ignored paths
	if len(e.Ignored) == 0 {
		e.Ignored = []string{".git", "tests", "node_modules/.cache", "vendor/bin"}
	}

	return nil
}

// GetEnvironment retrieves a specific environment configuration
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	env, ok := c.Environments[name]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not found in configuration", name)
	}
	return &env, nil
}

// interpolateEnvVars replaces ${VAR} or $VAR with environment variable values
func interpolateEnvVars(content string) string {
	return os.Expand(content, os.Getenv)
}
