package deployer

import (
	"testing"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/provision"
)

// With ssh.user root, services defaulted to root must move to the deploy user that
// gets the sudoers rules; an explicit root service is refused.
func TestServicesForDeployUserNeverRoot(t *testing.T) {
	env := &config.Environment{RemotePath: "/var/www/app", SSH: config.SSHConfig{User: "root"}}
	env.Services = []config.ServiceConfig{{Name: "app", User: "root"}, {Name: "worker", User: "www-data"}}
	v := provision.Vars{DeployUser: "deploy"}

	svcs, err := ServicesForDeployUser(env, v)
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
	if _, err := ServicesForDeployUser(env, v); err == nil {
		t.Fatal("a root service with a non-root deploy user must be refused")
	}
}
