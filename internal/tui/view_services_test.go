package tui

import (
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/provision"
)

func TestServicesViewRendersAndSelects(t *testing.T) {
	env := &config.Environment{RemotePath: "/var/www/app", SSH: config.SSHConfig{Host: "h", User: "deploy", Port: 22}}
	env.Builds.PHP.Enabled = true
	s := newServicesModel(120, 40)
	r := provision.ParseReport("apt=1\nos_id=ubuntu\nsvc_nginx=active 1.24.0\nupg nginx/noble-security 1.24.1 amd64 [upgradable from: 1.24.0]\nreboot=1")
	s.applyDetected(msgServicesDetected{envName: "prod", report: r}, env, "app", t.TempDir())

	out := s.view(120, 40, env)
	for _, want := range []string{"Nginx", "security update", "reboot required", "Backend: PHP", "Parameters"} {
		if !strings.Contains(out, want) {
			t.Errorf("view misses %q", want)
		}
	}
	if s.selected["nginx"] {
		t.Error("active nginx should not be preselected")
	}

	pick := func(s *servicesModel, id string) {
		for i, svc := range provision.Catalog {
			if svc.ID == id {
				s.cursor = i
				s.toggle()
			}
		}
	}
	// nginx is installed: Apache can't be selected
	pick(&s, "apache")
	if s.selected["apache"] || !strings.Contains(s.status, "nginx is already installed") {
		t.Errorf("apache selectable next to nginx: %v %q", s.selected, s.status)
	}

	// clean server: selecting apache deselects nginx (same group) and switches the web server
	clean := newServicesModel(120, 40)
	clean.applyDetected(msgServicesDetected{envName: "prod", report: provision.ParseReport("apt=1")}, env, "app", t.TempDir())
	pick(&clean, "nginx")
	pick(&clean, "apache")
	if clean.selected["nginx"] || !clean.selected["apache"] || clean.vars.WebServer != "apache" {
		t.Errorf("exclusive group broken: %v web=%s", clean.selected, clean.vars.WebServer)
	}

	s.askUpgrade("security")
	if s.confirmTitle == "" || s.pending == nil {
		t.Errorf("security upgrade not prepared: %q", s.status)
	}
	if out := s.view(120, 40, env); !strings.Contains(out, "nginx") || !strings.Contains(out, "[Y]") {
		t.Error("confirm modal not rendered")
	}
}

func TestVersionChoices(t *testing.T) {
	if got := versionChoices("python", "3.13"); got[0] != "" || len(got) < 4 {
		t.Errorf("python choices must start with system and offer 3+: %v", got)
	}
	if got := versionChoices("php", "8.1"); got[len(got)-1] != "8.1" {
		t.Errorf("current version missing from choices: %v", got)
	}
	s := newServicesModel(120, 40)
	s.vars.NodeMajor = provision.Versions("node")[0]
	for i, f := range svcFields {
		if f.product == "node" {
			s.cursor = len(provision.Catalog) + i
		}
	}
	if !s.cycleVersion() || s.vars.NodeMajor != provision.Versions("node")[1] {
		t.Errorf("Enter should move to the next version, got %q", s.vars.NodeMajor)
	}
}

// With ssh.user root, services defaulted to root must move to the deploy user that
// gets the sudoers rules; an explicit root service is refused.
func TestServicesForDeployUserNeverRoot(t *testing.T) {
	env := &config.Environment{RemotePath: "/var/www/app", SSH: config.SSHConfig{User: "root"}}
	env.Services = []config.ServiceConfig{{Name: "app", User: "root"}, {Name: "worker", User: "www-data"}}
	v := provision.Vars{DeployUser: "deploy"}

	svcs, err := servicesForDeployUser(env, v)
	if err != nil {
		t.Fatal(err)
	}
	if svcs[0].User != "deploy" || svcs[1].User != "www-data" {
		t.Fatalf("users = %s, %s", svcs[0].User, svcs[1].User)
	}
	if env.Services[0].User != "root" {
		t.Fatal("the environment's services were modified")
	}

	env.SSH.User = "deploy"
	env.Services = []config.ServiceConfig{{Name: "app", User: "root"}}
	if _, err := servicesForDeployUser(env, v); err == nil {
		t.Fatal("a root service with a non-root deploy user must be refused")
	}
}
