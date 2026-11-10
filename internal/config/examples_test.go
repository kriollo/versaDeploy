package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestExamples: every examples/deploy.*.yml loads, validates, and has no unknown
// (misspelled) keys.
func TestExamples(t *testing.T) {
	files, _ := filepath.Glob("../../examples/deploy.*.yml")
	if len(files) < 6 {
		t.Fatalf("examples: %v", files)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	os.MkdirAll(filepath.Join(home, ".ssh"), 0700)
	os.WriteFile(filepath.Join(home, ".ssh/id_ed25519"), []byte("key"), 0600)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			cfg, err := Load(f)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(f)
			dec := yaml.NewDecoder(bytes.NewReader(data))
			dec.KnownFields(true)
			if err := dec.Decode(&Config{}); err != nil {
				t.Errorf("unknown key: %v", err)
			}
			env, err := cfg.GetEnvironment("production")
			// the server's secrets: .env, or versaWYS's config.json (versaWYS/config)
			if err != nil || len(env.SharedPaths) == 0 || (env.SharedPaths[0] != ".env" && env.SharedPaths[0] != "versaWYS/config") {
				t.Errorf("production must share .env (versaWYS: versaWYS/config): %v %v", env, err)
			}
		})
	}
}
