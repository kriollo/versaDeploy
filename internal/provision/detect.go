package provision

import (
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"time"

	versassh "github.com/user/versaDeploy/internal/ssh"
)

// Service states reported by detection.
const (
	Absent    = "absent"
	Installed = "installed" // present; for daemons: not running
	Active    = "active"    // daemon running
)

// State is the detected state of one service.
type State struct {
	Status  string
	Version string
}

// Upgrade is one pending apt upgrade.
type Upgrade struct {
	Pkg, From, To string
	Security      bool
}

// Report is everything one detection round-trip learns about the server.
type Report struct {
	Services       map[string]State
	PHPVersion     string // php-fpm version found on the server, e.g. "8.4"
	Apt            bool   // apt-get available: installs and upgrades are supported
	Dnf            bool   // dnf available (RHEL family 8+): installs are supported, upgrades aren't
	Yum            bool   // yum available (also on dnf servers, as an alias)
	OSID           string // ID from /etc/os-release
	Upgrades       []Upgrade
	RebootRequired bool
	RebootPkgs     []string
	Unattended     string // active | inactive | "" (not installed)
	AptAge         int64  // seconds since the last apt index refresh, -1 unknown
	Vhost          string // nginx | apache when a vhost for the project exists
	Sites          []Site // vhosts enabled on the server (every app it hosts)
	BackupLast     string // newest dump in remote_path/backups, "name size"
	ProjectSite    string // site file serving remote_path when not named after the project
}

// Site is one enabled vhost.
type Site struct {
	File  string
	Names string // server_name / ServerName
	Root  string
}

// DetectParams names what the probe looks for.
type DetectParams struct {
	Project    string
	RemotePath string
	DeployUser string
	AppService string
}

// detectScript avoids non-POSIX tools so it runs on old servers. sbin is added to PATH
// because nginx/apache2/php-fpm live there and the deploy user's PATH usually lacks it.
const detectScript = `PATH=$PATH:/usr/sbin:/sbin:/usr/local/sbin
has() { command -v "$1" >/dev/null 2>&1; }
svc() { # id "units" version present
  if [ "$4" != 1 ]; then echo "svc_$1=absent $3"; return; fi
  s=installed
  for u in $2; do
    if [ -d /run/systemd/system ]; then systemctl is-active --quiet "$u" 2>/dev/null && s=active
    elif service "$u" status >/dev/null 2>&1; then s=active; fi
  done
  echo "svc_$1=$s $3"
}
[ -f /etc/os-release ] && echo "os_id=$(. /etc/os-release; echo $ID)"
has apt-get && echo "apt=1"
has dnf && echo "dnf=1"
has yum && echo "yum=1"
has curl && has git && has unzip && echo "svc_base=installed"
has nginx && svc nginx nginx "$(nginx -v 2>&1 | sed 's|.*/||')" 1
if has apache2; then svc apache apache2 "$( (apache2ctl -v || apache2 -v) 2>/dev/null | sed -n 's|^Server version: Apache/\([^ ]*\).*|\1|p')" 1
elif has httpd; then svc apache httpd "$(httpd -v 2>/dev/null | sed -n 's|^Server version: Apache/\([^ ]*\).*|\1|p')" 1; fi
fpm=""
for v in 8.5 8.4 8.3 8.2 8.1 8.0 7.4; do if has php-fpm$v; then fpm=$v; break; fi; done
if [ -n "$fpm" ]; then echo "php_fpm=$fpm"; svc php "php$fpm-fpm" "$(php-fpm$fpm -v 2>/dev/null | head -1 | awk '{print $2}')" 1
elif has php-fpm; then fv=$(php-fpm -v 2>/dev/null | head -1 | awk '{print $2}'); echo "php_fpm=$(echo "$fv" | cut -d. -f1,2)"; svc php php-fpm "$fv" 1
elif has php; then echo "svc_php=absent $(php -v 2>/dev/null | head -1 | awk '{print $2}') (cli only)"; fi
if has certbot; then echo "svc_ssl=installed $(certbot --version 2>&1 | awk '{print $2}')"
elif ls /etc/ssl/versa/*/fullchain.pem >/dev/null 2>&1; then echo "svc_ssl=installed self-signed"; fi
pgv=$(psql --version 2>/dev/null | awk '{print $3}')
if ls -d /usr/lib/postgresql/*/bin/postgres >/dev/null 2>&1 || has postgres || has postmaster; then svc postgres postgresql "$pgv" 1; fi
if has mariadbd || has mysqld; then svc mariadb "mariadb mysql mysqld" "$(mysql --version 2>/dev/null | sed 's/^mysql *//')" 1; fi
has redis-server && svc redis "redis-server redis" "$(redis-server --version 2>/dev/null | sed -n 's/.*v=\([^ ]*\).*/\1/p')" 1
# dragonfly --version is colored: "dragonfly ESC[0;32mv2.1-<sha>ESC[m"
{ has dragonfly || [ -x /usr/local/bin/dragonfly ]; } && svc dragonfly dragonfly "$( (dragonfly --version || /usr/local/bin/dragonfly --version) 2>/dev/null | head -1 | awk '{print $2}' | tr -d '\033' | sed 's/\[[0-9;]*m//g; s/-.*//')" 1
if has python3; then pv=$(python3 -V 2>&1 | awk '{print $2}')
  if python3 -c 'import ensurepip' >/dev/null 2>&1; then echo "svc_python=installed $pv"; else echo "svc_python=absent $pv (no venv)"; fi; fi
has node && echo "svc_node=installed $(node -v 2>/dev/null)"
if [ -n "$app" ] && [ -f "/etc/systemd/system/$app.service" ]; then svc app_service "$app" "$app" 1; fi
[ -n "$user" ] && id "$user" >/dev/null 2>&1 && echo "svc_deploy_user=installed $user"
if [ -n "$user" ] && { ls /etc/sudoers.d/"$user"* >/dev/null 2>&1 || { [ "$(id -un)" = "$user" ] && sudo -n -l 2>/dev/null | grep -q systemctl; }; }; then echo "svc_sudoers=installed"; fi
[ -d "$p" ] && echo "svc_dirs=installed $p"
has ufw && svc ufw ufw "" 1
if [ -n "$proj" ] && [ -f "/etc/cron.d/versa-$(echo "$proj" | tr . _)" ]; then echo "svc_cron=installed $(grep -c "^[@0-9*]" "/etc/cron.d/versa-$(echo "$proj" | tr . _)") job(s)"; fi
if [ -n "$proj" ] && [ -f "/etc/systemd/system/versa-backup-$proj.timer" ]; then svc backup "versa-backup-$proj.timer" "" 1; fi
if [ -d "$p/backups" ]; then b=$(ls -1t "$p/backups" 2>/dev/null | head -1); [ -n "$b" ] && echo "backup_last=$b $(du -h "$p/backups/$b" 2>/dev/null | cut -f1)"; fi
for f in /etc/nginx/sites-enabled/* /etc/apache2/sites-enabled/* /etc/nginx/conf.d/*.conf; do
  [ -f "$f" ] || continue
  n=$(sed -n -e 's/^[[:space:]]*server_name[[:space:]]*\([^;]*\);.*/\1/p' -e 's/^[[:space:]]*ServerName[[:space:]]*\([^[:space:]]*\).*/\1/p' "$f" | head -1)
  case $f in /etc/nginx/conf.d/*) [ -n "$n" ] || continue ;; esac # RHEL's sites; skip global snippets
  r=$(sed -n -e 's/^[[:space:]]*root[[:space:]]*\([^;]*\);.*/\1/p' -e 's/^[[:space:]]*DocumentRoot[[:space:]]*\([^[:space:]]*\).*/\1/p' "$f" | head -1)
  echo "site=$(basename "$f")|$n|$r"
done
if [ -n "$proj" ]; then
  if [ -f "/etc/nginx/sites-available/$proj" ] || [ -f "/etc/nginx/conf.d/$proj.conf" ]; then echo "vhost=nginx"; elif [ -f "/etc/apache2/sites-available/$proj.conf" ]; then echo "vhost=apache"; fi
fi
if has apt-get; then
  apt list --upgradable 2>/dev/null | sed -n 's|^\([^ ]*/.*upgradable.*\)$|upg \1|p'
  [ -f /var/run/reboot-required ] && echo "reboot=1" && echo "reboot_pkgs=$(cat /var/run/reboot-required.pkgs 2>/dev/null | tr '\n' ' ')"
  [ -f /usr/bin/unattended-upgrade ] && if systemctl is-active --quiet unattended-upgrades 2>/dev/null; then echo "unattended=active"; else echo "unattended=inactive"; fi
  [ -f /var/cache/apt/pkgcache.bin ] && echo "apt_age=$(( $(date +%s) - $(stat -c %Y /var/cache/apt/pkgcache.bin) ))"
fi
true`

// DetectScript returns the probe script for p.
func DetectScript(p DetectParams) string {
	q := versassh.ShellQuote
	return "proj=" + q(p.Project) + "; p=" + q(p.RemotePath) + "; user=" + q(p.DeployUser) + "; app=" + q(p.AppService) + "\n" + detectScript
}

// Detect probes the server in one round-trip.
func Detect(client *versassh.Client, p DetectParams) (*Report, error) {
	out, err := client.ExecuteCommand(DetectScript(p))
	if err != nil && out == "" {
		return nil, err
	}
	r := ParseReport(out)
	r.matchProjectSite(p.RemotePath)
	return r, nil
}

// matchProjectSite finds a vhost made by hand or by certbot under another name, by its
// document root.
func (r *Report) matchProjectSite(remotePath string) {
	if r.Vhost != "" || remotePath == "" {
		return
	}
	for _, s := range r.Sites {
		if s.Root == remotePath || strings.HasPrefix(s.Root, strings.TrimSuffix(remotePath, "/")+"/") {
			r.Vhost = siteServer(s.File, r)
			r.ProjectSite = s.File
			return
		}
	}
}

// siteServer guesses which web server owns a site file.
func siteServer(file string, r *Report) string {
	if strings.HasSuffix(file, ".conf") && r.Get("apache").Status != Absent {
		return "apache"
	}
	if r.Get("nginx").Status != Absent {
		return "nginx"
	}
	return "apache"
}

// CanInstall reports whether the install scripts support this server's package manager.
func (r *Report) CanInstall() bool { return r.Apt || r.Dnf }

// ParseReport parses the probe output.
func ParseReport(out string) *Report {
	r := &Report{Services: map[string]State{}, AptAge: -1}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		// the server controls this text and the TUI prints it: no escape sequences
		line = strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' || r == 0x7f || r >= 0x80 && r < 0xa0 {
				return -1
			}
			return r
		}, line)
		if rest, ok := strings.CutPrefix(line, "upg "); ok {
			if u, ok := parseUpgrade(rest); ok {
				r.Upgrades = append(r.Upgrades, u)
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if id, ok := strings.CutPrefix(k, "svc_"); ok {
			st, ver, _ := strings.Cut(v, " ")
			r.Services[id] = State{Status: st, Version: strings.TrimSpace(ver)}
			continue
		}
		switch k {
		case "os_id":
			r.OSID = v
		case "apt":
			r.Apt = v == "1"
		case "dnf":
			r.Dnf = v == "1"
		case "yum":
			r.Yum = v == "1"
		case "php_fpm":
			if phpVersionRe.MatchString(v) { // ends up in unit names of root scripts
				r.PHPVersion = v
			}
		case "reboot":
			r.RebootRequired = v == "1"
		case "reboot_pkgs":
			r.RebootPkgs = strings.Fields(v)
		case "unattended":
			r.Unattended = v
		case "apt_age":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				r.AptAge = n
			}
		case "vhost":
			r.Vhost = v
		case "backup_last":
			r.BackupLast = v
		case "site":
			f := strings.SplitN(v, "|", 3)
			for len(f) < 3 {
				f = append(f, "")
			}
			r.Sites = append(r.Sites, Site{File: f[0], Names: strings.TrimSpace(f[1]), Root: strings.TrimSpace(f[2])})
		}
	}
	return r
}

// parseUpgrade parses one `apt list --upgradable` line:
// nginx/noble-updates,noble-security 1.24.0-2ubuntu7.1 amd64 [upgradable from: 1.24.0-2ubuntu7]
func parseUpgrade(line string) (Upgrade, bool) {
	f := strings.Fields(line)
	if len(f) < 2 {
		return Upgrade{}, false
	}
	pkg, suites, ok := strings.Cut(f[0], "/")
	if !ok || !debPkgRe.MatchString(pkg) {
		return Upgrade{}, false
	}
	u := Upgrade{Pkg: pkg, To: f[1], Security: strings.Contains(suites, "-security")}
	if i := strings.Index(line, "from: "); i >= 0 {
		u.From = strings.TrimSuffix(strings.TrimSpace(line[i+6:]), "]")
	}
	return u, true
}

// Get returns the state of id, Absent when it wasn't reported.
func (r *Report) Get(id string) State {
	if s, ok := r.Services[id]; ok && s.Status != "" {
		return s
	}
	return State{Status: Absent}
}

// UpgradesFor returns the pending upgrades owned by service id.
func (r *Report) UpgradesFor(id string) []Upgrade {
	svc, ok := Lookup(id)
	if !ok {
		return nil
	}
	var out []Upgrade
	for _, u := range r.Upgrades {
		if svc.Owns(u.Pkg, r.PHPVersion) {
			out = append(out, u)
		}
	}
	return out
}

// SecurityUpgrades returns the pending security upgrades (any package).
func (r *Report) SecurityUpgrades() []Upgrade {
	var out []Upgrade
	for _, u := range r.Upgrades {
		if u.Security {
			out = append(out, u)
		}
	}
	return out
}

// CertExpiry checks the certificate a domain serves on :443, from this machine.
// It reports what visitors get, so it needs no access to /etc/letsencrypt.
func CertExpiry(domain string) (time.Time, error) {
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{ServerName: domain})
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates[0].NotAfter, nil
}
