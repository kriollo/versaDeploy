// Package provision detects and installs the server stack (web server, PHP, databases,
// runtimes...) for an environment. The install scripts are a non-interactive port of
// setup-vps.sh; they support apt (Debian/Ubuntu) and dnf (RHEL/Rocky/Alma 8+) servers.
// Detection is POSIX and runs anywhere.
package provision

import (
	"path"
	"strings"
)

// Service is one installable piece of the stack.
type Service struct {
	ID       string
	Label    string
	Group    string   // services sharing a group are mutually exclusive ("web", "db")
	Packages []string // apt package globs owned by the service; {v} = PHP version
	Units    []string // systemd units reloaded after an upgrade; {v} = PHP version
}

// Catalog lists the services in install order (same order as setup-vps.sh).
var Catalog = []Service{
	{ID: "base", Label: "Base packages (curl, git, unzip…)"},
	{ID: "php", Label: "PHP-FPM + OPCache", Packages: []string{"php{v}-*"}, Units: []string{"php{v}-fpm"}},
	{ID: "apache", Label: "Apache2", Group: "web", Packages: []string{"apache2", "apache2-*"}, Units: []string{"apache2"}},
	{ID: "nginx", Label: "Nginx", Group: "web", Packages: []string{"nginx", "nginx-*", "libnginx-mod-*"}, Units: []string{"nginx"}},
	{ID: "ssl", Label: "SSL (Let's Encrypt or self-signed)", Packages: []string{"certbot", "python3-certbot*"}},
	{ID: "postgres", Label: "PostgreSQL", Group: "db", Packages: []string{"postgresql", "postgresql-*"}, Units: []string{"postgresql"}},
	{ID: "mariadb", Label: "MariaDB / MySQL", Group: "db", Packages: []string{"mariadb-*", "mysql-*"}, Units: []string{"mariadb", "mysql"}},
	{ID: "redis", Label: "Redis", Group: "cache", Packages: []string{"redis-server", "redis-tools", "redis"}, Units: []string{"redis-server"}},
	{ID: "dragonfly", Label: "Dragonfly (Redis-compatible)", Group: "cache", Units: []string{"dragonfly"}},
	{ID: "python", Label: "Python 3 + venv", Packages: []string{"python3", "python3-pip", "python3-venv", "python3-dev"}},
	{ID: "node", Label: "Node.js", Packages: []string{"nodejs"}},
	{ID: "app_service", Label: "App service (deploy installs it)"},
	{ID: "deploy_user", Label: "Deploy user + SSH key"},
	{ID: "sudoers", Label: "Sudoers (services_reload)"},
	{ID: "dirs", Label: "Project directory"},
	{ID: "backup", Label: "DB backups (daily timer)"},
	{ID: "cron", Label: "Cron jobs (provision.cron)"},
	{ID: "ufw", Label: "UFW firewall", Packages: []string{"ufw"}},
}

// Lookup returns the catalog entry for id.
func Lookup(id string) (Service, bool) {
	for _, s := range Catalog {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

// Owns reports whether apt package pkg belongs to the service.
func (s Service) Owns(pkg, phpVersion string) bool {
	for _, g := range s.Packages {
		g = strings.ReplaceAll(g, "{v}", phpVersion)
		if ok, _ := path.Match(g, pkg); ok {
			return true
		}
	}
	return false
}

// UnitNames returns the service's systemd units with the PHP version filled in.
func (s Service) UnitNames(phpVersion string) []string {
	var out []string
	for _, u := range s.Units {
		out = append(out, strings.ReplaceAll(u, "{v}", phpVersion))
	}
	return out
}
