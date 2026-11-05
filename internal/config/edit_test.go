package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSaveProvisionKeepsCommentsAndPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.yml")
	src := `project: demo
# top comment
environments:
    prod:
        ssh:
            host: 1.2.3.4 # the server
        remote_path: /var/www/demo
        services_reload:
            - sudo systemctl reload nginx
        provision:
            domain: old.com
            db_password: ${DB_PASS}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	p := ProvisionConfig{Domain: "new.com", WebServer: "nginx", DBPassword: "leaked", SSH: ProvisionSSHConfig{User: "root"}}
	if err := SaveProvision(path, "prod", p); err != nil {
		t.Fatal(err)
	}
	if err := AddServicesReload(path, "prod", []string{"sudo systemctl reload nginx", "sudo systemctl reload php8.4-fpm"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	for _, want := range []string{"# top comment", "# the server", "domain: new.com", "db_password: ${DB_PASS}", "user: root", "        remote_path:"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "leaked") || strings.Contains(s, "old.com") {
		t.Errorf("unexpected content:\n%s", s)
	}

	var cfg Config
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Environments["prod"].ServicesReload; len(got) != 2 {
		t.Errorf("services_reload = %v, want 2 entries (no duplicate)", got)
	}
	if err := SaveProvision(path, "missing", p); err == nil {
		t.Error("expected error for unknown environment")
	}
}

func TestProvisionSSHInherits(t *testing.T) {
	e := Environment{SSH: SSHConfig{Host: "h", User: "deploy", KeyPath: "/k", Port: 2222}}
	e.Provision.SSH.User = "root"
	c := e.ProvisionSSH()
	if c.Host != "h" || c.User != "root" || c.KeyPath != "/k" || c.Port != 2222 {
		t.Errorf("ProvisionSSH() = %+v", c)
	}
	if e.SSH.User != "deploy" {
		t.Error("ProvisionSSH mutated env.SSH")
	}
}

func TestValidateExpandsKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	os.MkdirAll(filepath.Join(home, ".ssh"), 0700)
	os.WriteFile(filepath.Join(home, ".ssh", "id"), []byte("k"), 0600)
	e := Environment{SSH: SSHConfig{Host: "h", User: "u", KeyPath: "~/.ssh/id", KnownHostsFile: "~/.ssh/known_hosts"}, RemotePath: "/srv"}
	e.Builds.PHP.Enabled = true
	if err := e.Validate("prod"); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".ssh", "known_hosts"); e.SSH.KnownHostsFile != want {
		t.Errorf("known_hosts_file = %q, want %q", e.SSH.KnownHostsFile, want)
	}
}

func TestSetServices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.yml")
	os.WriteFile(path, []byte("project: demo\nenvironments:\n    prod:\n        remote_path: /srv\n"), 0644)
	if err := SetServices(path, "prod", []ServiceConfig{{Name: "api", Exec: "npm start", Environment: map[string]string{"PORT": "3000"}}}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	for _, want := range []string{"services:", "name: api", "exec: npm start", `PORT: "3000"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "start_wait") || strings.Contains(s, "working_dir") {
		t.Errorf("empty fields written:\n%s", s)
	}
}
