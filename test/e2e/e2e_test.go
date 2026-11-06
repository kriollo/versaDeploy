//go:build e2e

// End-to-end test of the Services view backend: provisions Docker containers with the
// install scripts and deploys real apps with versaDeploy.
//
//   - ubuntu:24.04 (nginx + PostgreSQL): four apps on one server, one per backend
//     (test/e2e/apps/{php,python,go,node}), each with its own domain, vhost and unit
//   - debian:12 (Apache + MariaDB): versaTodo (../versaTodo, skipped when missing)
//
// It also checks DB backups (timer + backup now), service logs and the sites listing.
// Needs Docker able to run systemd containers:
//
//	go test -tags e2e -v -timeout 60m ./test/e2e/
//
// VERSA_E2E_REPO overrides the versaTodo path, VERSA_E2E_KEEP=1 leaves the containers.
package e2e

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/provision"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type server struct {
	name, image       string
	sshPort, httpPort int
	web, db           string
	stack             []string // server-wide steps, run once before the first app
}

type app struct {
	name     string // project name, env name and domain prefix
	server   *server
	repo     string // filled at runtime
	builds   string // YAML for builds:
	services string // YAML list for services: ("" = none)
	tweak    func(v *provision.Vars)
	steps    []string // per-app steps
	expect   string   // body marker; "" = versaTodo installer rules
}

var (
	ubuntu = &server{"ubuntu", "ubuntu:24.04", 2222, 8080, "nginx", "postgres",
		[]string{"base", "php", "nginx", "postgres", "redis", "python", "node", "deploy_user"}}
	debian = &server{"debian", "debian:12", 2223, 8081, "apache", "mariadb",
		[]string{"base", "php", "apache", "mariadb", "redis", "deploy_user"}}
)

var apps = []app{
	{name: "e2ephp", server: ubuntu, expect: "versa-e2e php",
		builds: `php: { enabled: true, root: '.', composer_command: 'composer install --no-dev' }`,
		steps:  []string{"nginx", "sudoers", "dirs"}},
	{name: "e2epython", server: ubuntu, expect: "versa-e2e python",
		builds: `python: { enabled: true, web_server: true, web_port: 8000, entry_point: app.py, service_name: e2epython }`,
		tweak:  func(v *provision.Vars) { v.Backend, v.ProxyPort = "python", 8000 },
		// owns the server's PostgreSQL database and its daily backup
		services: `[{ name: e2epython }]`, // exec: the builder's run_server.sh
		steps:    []string{"nginx", "postgres", "sudoers", "dirs", "backup"}},
	{name: "e2ego", server: ubuntu, expect: "versa-e2e go",
		builds:   `go: { enabled: true, target_os: linux, target_arch: amd64, binary_name: app, deploy_path: bin }`,
		tweak:    func(v *provision.Vars) { v.Backend, v.ProxyPort = "go", 8090 },
		services: `[{ name: e2ego, environment: { PORT: "8090" } }]`, // exec: the go binary
		steps:    []string{"nginx", "sudoers", "dirs"}},
	{name: "e2enode", server: ubuntu, expect: "versa-e2e node",
		builds:   `frontend: { enabled: true, npm_command: 'npm install', compile_command: 'npm run build' }`,
		tweak:    func(v *provision.Vars) { v.Backend, v.ProxyPort, v.NodeMajor = "node", 3000, "22" },
		services: `[{ name: e2enode, exec: "npm start", environment: { PORT: "3000" } }]`,
		steps:    []string{"nginx", "sudoers", "dirs"}},
	{name: "versatodo", server: debian,
		builds: `php: { enabled: true, root: '.', composer_command: 'composer install --no-dev --optimize-autoloader' }
            frontend: { enabled: true, npm_command: 'pnpm install --frozen-lockfile', compile_command: 'pnpm run compile', cleanup_dev_deps: true, production_command: 'pnpm install --production' }`,
		steps: []string{"apache", "mariadb", "sudoers", "dirs", "backup"}},
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// prefixWriter tags output lines with the server name.
type prefixWriter struct {
	mu     *sync.Mutex
	w      io.Writer
	prefix string
}

func (p prefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, l := range strings.Split(strings.TrimRight(strings.ReplaceAll(string(b), "\r", ""), "\n"), "\n") {
		fmt.Fprintf(p.w, "[%s] %s\n", p.prefix, l)
	}
	return len(b), nil
}

// gitRepo copies a fixture into a fresh git repo (the deployer clones the repo).
func gitRepo(t *testing.T, src string) string {
	dst := t.TempDir()
	run(t, "cp", "-a", src+"/.", dst)
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.email=e2e@versa", "-c", "user.name=e2e", "commit", "-qm", "e2e"}} {
		c := exec.Command("git", args...)
		c.Dir = dst
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dst
}

func (a app) domain() string { return a.name + ".test" }

func (a app) vars(pub string) provision.Vars {
	v := provision.Vars{
		Project: a.name, Domain: a.domain(), WebServer: a.server.web, PHPVersion: "8.4",
		RemotePath: "/var/www/versa/" + a.name, DeployUser: "versadeploy",
		Database: a.server.db, DBName: a.name, DBUser: a.name + "_user", DBPassword: "e2e'pass",
		SSHPort: 22, PubKey: pub, Backend: "php", Redis: true, BackupKeep: 3,
		Aliases: []string{"www." + a.domain()},
	}
	if a.tweak != nil {
		a.tweak(&v)
	}
	if v.Backend != "php" {
		v.AppService = a.name // deploy.yml services name: installed by the deploy
	}
	return v
}

func get(t *testing.T, port int, host, path string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	req.Host = host
	c := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// app units may need a moment after services_reload restarted them
	var lastErr error
	for i := 0; i < 10; i++ {
		resp, err := c.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == 502 && i < 9 {
			time.Sleep(time.Second)
			continue
		}
		return resp.StatusCode, string(body)
	}
	t.Fatalf("GET %s%s: %v", host, path, lastErr)
	return 0, ""
}

func TestProvisionAndDeploy(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	todo := os.Getenv("VERSA_E2E_REPO")
	if todo == "" {
		todo = "../../../versaTodo"
	}
	todo, _ = filepath.Abs(todo)
	var selected []app
	for _, a := range apps {
		if a.name == "versatodo" {
			if _, err := os.Stat(filepath.Join(todo, "composer.json")); err != nil {
				t.Logf("versaTodo not found at %s: skipping it", todo)
				continue
			}
			a.repo = todo
		} else {
			a.repo = gitRepo(t, filepath.Join("apps", strings.TrimPrefix(a.name, "e2e")))
		}
		selected = append(selected, a)
	}
	servers := []*server{ubuntu}
	for _, a := range selected {
		if a.server == debian {
			servers = append(servers, debian)
		}
	}

	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	run(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	pubBytes, _ := os.ReadFile(key + ".pub")
	pub := string(pubBytes)
	ctx := filepath.Join(dir, "ctx")
	os.MkdirAll(ctx, 0755)
	os.WriteFile(filepath.Join(ctx, "id.pub"), pubBytes, 0644)
	df, _ := filepath.Abs("Dockerfile")

	knownHosts := filepath.Join(dir, "known_hosts")
	for _, s := range servers {
		img := "versa-e2e-" + s.name
		run(t, "docker", "build", "-q", "-t", img, "--build-arg", "BASE="+s.image, "-f", df, ctx)
		exec.Command("docker", "rm", "-f", img).Run()
		run(t, "docker", "run", "-d", "--name", img, "--privileged", "--cgroupns=host",
			"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock",
			"-p", fmt.Sprintf("127.0.0.1:%d:22", s.sshPort), "-p", fmt.Sprintf("127.0.0.1:%d:80", s.httpPort), img)
		if os.Getenv("VERSA_E2E_KEEP") == "" {
			t.Cleanup(func() { exec.Command("docker", "rm", "-f", img).Run() })
		}
		// register the host key, as a user would with ssh-keyscan
		var keys []byte
		for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline) && len(keys) == 0; time.Sleep(2 * time.Second) {
			keys, _ = exec.Command("ssh-keyscan", "-p", fmt.Sprint(s.sshPort), "127.0.0.1").Output()
		}
		if len(keys) == 0 {
			t.Fatalf("%s: sshd not reachable", s.name)
		}
		f, _ := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		f.Write(keys)
		f.Close()
	}

	log, _ := logger.NewLogger("", true, false)
	sshCfg := func(s *server, user string) config.SSHConfig {
		return config.SSHConfig{Host: "127.0.0.1", Port: s.sshPort, User: user, KeyPath: key, KnownHostsFile: knownHosts, StrictHostKey: true}
	}
	rootClient := func(s *server) *versassh.Client {
		cfg := sshCfg(s, "root")
		var c *versassh.Client
		var err error
		for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
			if c, err = versassh.NewClient(&cfg, log); err == nil {
				break
			}
		}
		if err != nil {
			t.Fatalf("%s ssh: %v", s.name, err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}

	// The deploy config, with the services_reload the view suggests; the sudoers step
	// installs the rules its services need (as the Services view does)
	cfgPath := filepath.Join(dir, "deploy_e2e.yml")
	var y strings.Builder
	y.WriteString("project: e2e\nenvironments:\n")
	for _, a := range selected {
		v := a.vars(pub)
		var reload strings.Builder
		for _, c := range provision.SuggestedReload(v) {
			fmt.Fprintf(&reload, "            - '%s'\n", c)
		}
		fmt.Fprintf(&y, `    %s:
        ssh: { host: 127.0.0.1, port: %d, user: versadeploy, key_path: %s, known_hosts_file: %s, strict_host_key: true }
        remote_path: %s
        ignored_paths: ['.git', 'node_modules/.cache', 'versaWYS/tests', 'documentacion', 'src']
        shared_paths: ['versaWYS/logs', 'versaWYS/cache']
        builds:
            %s
        services_reload:
%s`, a.name, a.server.sshPort, key, knownHosts, v.RemotePath, a.builds, reload.String())
		if a.services != "" {
			fmt.Fprintf(&y, "        services: %s\n", a.services)
		}
	}
	os.WriteFile(cfgPath, []byte(y.String()), 0644)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config: %v\n%s", err, y.String())
	}
	fullVars := func(a app) provision.Vars {
		v := a.vars(pub)
		if env, err := cfg.GetEnvironment(a.name); err == nil && len(env.Services) > 0 {
			v.ServicesSudoers = deployer.SudoersScript(env, cfg.Project)
			inst, err := deployer.ServiceInstallScript(env, cfg.Project, a.name)
			if err != nil {
				t.Fatal(err)
			}
			v.ServicesInstall = inst
		}
		return v
	}

	// 1. Provision: per server the stack once, then each app's steps (servers in parallel)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := map[string]error{}
	for _, s := range servers {
		c := rootClient(s)
		wg.Add(1)
		go func() {
			defer wg.Done()
			stack := s.stack
			for _, a := range selected {
				if a.server != s {
					continue
				}
				steps := append(append([]string{}, stack...), a.steps...)
				stack = nil
				b, err := provision.Build(steps, fullVars(a))
				if err == nil {
					err = provision.Run(c, "root", b, prefixWriter{&mu, os.Stdout, s.name + "/" + a.name})
				}
				if err != nil {
					mu.Lock()
					errs[a.name] = err
					mu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()
	for name, err := range errs {
		t.Fatalf("provision %s: %v", name, err)
	}

	// 1b. Re-running on a server in use changes nothing that is already there: existing DB
	// users keep their password, services aren't restarted, php.ini edits survive. Only an
	// explicit reset changes the password.
	for _, s := range servers {
		c := rootClient(s)
		sh := func(cmd string) string {
			out, err := c.ExecuteCommand(cmd)
			if err != nil {
				t.Fatalf("%s: %s: %v\n%s", s.name, cmd, err, out)
			}
			return strings.TrimSpace(out)
		}
		var owner app // the app that owns the server's database
		for _, a := range selected {
			if a.server == s && contains(a.steps, s.db) {
				owner = a
			}
		}
		v := fullVars(owner)
		login := map[string]string{
			"postgres": "PGPASSWORD=%s psql -h 127.0.0.1 -U " + v.DBUser + " -d " + v.DBName + " -tAc 'select 1'",
			"mariadb":  "mysql -h localhost -u " + v.DBUser + " -p%s " + v.DBName + " -N -e 'select 1'",
		}[s.db]
		canLogin := func(pass string) bool {
			out, _ := c.ExecuteCommand(fmt.Sprintf(login, versassh.ShellQuote(pass)) + " 2>&1")
			return strings.TrimSpace(out) == "1"
		}
		if !canLogin(v.DBPassword) {
			t.Fatalf("%s: %s cannot log in after provisioning", s.name, v.DBUser)
		}
		// pgrep arguments matching each daemon's main process
		daemons := map[string]string{"redis": "-x redis-server", "php": "-f 'php-fpm: master'",
			s.web: map[string]string{"nginx": "-x nginx", "apache": "-x apache2"}[s.web],
			s.db:  map[string]string{"postgres": "-x postgres", "mariadb": "-x mariadbd"}[s.db]}
		pids := map[string]string{}
		for name, match := range daemons {
			pids[name] = sh("pgrep -o " + match)
		}
		sh("sed -i 's/^memory_limit = .*/memory_limit = 512M/' /etc/php/8.4/fpm/php.ini")

		v.DBPassword = "another-pass"
		steps := append(append([]string{}, s.stack...), owner.steps...)
		b, err := provision.Build(steps, v)
		if err == nil {
			err = provision.Run(c, "root", b, prefixWriter{&mu, os.Stdout, s.name + "/rerun"})
		}
		if err != nil {
			t.Fatalf("%s: re-run: %v", s.name, err)
		}
		if !canLogin(owner.vars(pub).DBPassword) || canLogin("another-pass") {
			t.Errorf("%s: re-run changed the password of existing %s", s.name, v.DBUser)
		}
		for name, match := range daemons {
			if now := sh("pgrep -o " + match); now != pids[name] {
				t.Errorf("%s: %s was restarted by the re-run (pid %s → %s)", s.name, name, pids[name], now)
			}
		}
		if got := sh("grep '^memory_limit' /etc/php/8.4/fpm/php.ini"); got != "memory_limit = 512M" {
			t.Errorf("%s: php.ini overwritten: %q", s.name, got)
		}

		v.ResetDBPassword = true
		b, err = provision.Build([]string{s.db}, v)
		if err == nil {
			err = provision.Run(c, "root", b, prefixWriter{&mu, os.Stdout, s.name + "/reset"})
		}
		if err != nil || !canLogin("another-pass") {
			t.Errorf("%s: explicit reset did not set the new password: %v", s.name, err)
		}
		// back to the password the deploy config and app expect
		v.DBPassword = owner.vars(pub).DBPassword
		if b, err = provision.Build([]string{s.db}, v); err == nil {
			err = provision.Run(c, "root", b, io.Discard)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	// 2. Deploy every app as the deploy user: services are installed by the deploy
	for _, a := range selected {
		d, err := deployer.NewDeployer(cfg, a.name, a.repo, false, true, true, true, log)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Deploy(); err != nil {
			t.Fatalf("%s deploy: %v", a.name, err)
		}
	}

	// 3. Every app answers on its own domain and alias, through its web server
	for _, a := range selected {
		for _, host := range []string{a.domain(), "www." + a.domain()} {
			status, body := get(t, a.server.httpPort, host, "/")
			t.Logf("%s: GET http://%s/ → %d %q", a.name, host, status, firstLine(body))
			switch {
			case a.expect != "":
				if status != 200 || !strings.Contains(body, a.expect) {
					t.Errorf("%s (%s): got %d %q, want 200 with %q", a.name, host, status, firstLine(body), a.expect)
				}
			case status == 404 || status == 502 || status == 504 || (status >= 500 && !strings.Contains(body, "Instalación")):
				// versaTodo answers 503 with its installer until configured: still web server → PHP → app
				t.Errorf("%s: GET / = %d\n%s", a.name, status, body)
			}
		}
		if a.vars(pub).ProxyPort == 0 {
			// what versaWYS's .htaccess blocks (the vhost has AllowOverride None)
			for _, f := range []string{"/composer.json", "/versaCLI", "/pnpm-lock.yaml", "/README.md", "/tsconfig.json"} {
				if _, err := os.Stat(filepath.Join(a.repo, f)); err != nil {
					continue // not in this app: the router answers that path
				}
				if status, _ := get(t, a.server.httpPort, a.domain(), f); status == 200 {
					t.Errorf("%s: %s = 200, want it blocked", a.name, f)
				}
			}
		}
	}

	// 4. Detection sees all sites, units and backups; logs and backup-now work
	for _, s := range servers {
		c := rootClient(s)
		var hosted []app
		for _, a := range selected {
			if a.server == s {
				hosted = append(hosted, a)
			}
		}
		for _, a := range hosted {
			v := a.vars(pub)
			r, err := provision.Detect(c, provision.DetectParams{Project: a.name, RemotePath: v.RemotePath, DeployUser: "versadeploy", AppService: v.AppService})
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Sites) != len(hosted) {
				t.Errorf("%s: sites = %+v, want %d", s.name, r.Sites, len(hosted))
			}
			// the TUI detects over the deploy connection: vhosts must be readable by it
			dcfg := sshCfg(s, "versadeploy")
			if dc, err := versassh.NewClient(&dcfg, log); err != nil {
				t.Errorf("%s: deploy ssh: %v", s.name, err)
			} else {
				dr, err := provision.Detect(dc, provision.DetectParams{Project: a.name, RemotePath: v.RemotePath, DeployUser: "versadeploy"})
				dc.Close()
				for _, site := range dr.Sites {
					if err != nil || site.Names == "" || site.Root == "" {
						t.Errorf("%s: deploy user can't read vhost %s: %+v %v", s.name, site.File, site, err)
					}
				}
			}
			want := []string{s.web, "php", s.db, "redis"}
			if v.AppService != "" {
				want = append(want, "app_service")
			}
			if contains(a.steps, "backup") {
				want = append(want, "backup")
				if r.BackupLast == "" {
					t.Errorf("%s: no backup file after install", a.name)
				}
			}
			for _, id := range want {
				if st := r.Get(id); st.Status != provision.Active {
					t.Errorf("%s: %s = %+v, want active", a.name, id, st)
				}
			}
			for _, id := range []string{"base", "deploy_user", "sudoers", "dirs"} {
				if r.Get(id).Status == provision.Absent {
					t.Errorf("%s: %s not detected", a.name, id)
				}
			}
		}
		// never install an alternative web server: the script aborts before apt-get
		other := map[string]string{"nginx": "apache", "apache": "nginx"}[s.web]
		v := hosted[0].vars(pub)
		v.WebServer = other
		if b, err := provision.Build([]string{other}, v); err != nil {
			t.Fatal(err)
		} else {
			var out bytes.Buffer
			if err := provision.Run(c, "root", b, &out); err == nil || !strings.Contains(out.String(), "is already installed") {
				t.Errorf("%s: installing %s next to %s was not refused: %v\n%s", s.name, other, s.web, err, out.String())
			}
			if st, _ := c.ExecuteCommand("dpkg -l " + map[string]string{"nginx": "nginx", "apache": "apache2"}[other] + " 2>/dev/null | grep -c '^ii' || true"); strings.TrimSpace(st) != "0" {
				t.Errorf("%s: %s got installed anyway", s.name, other)
			}
		}

		var logs bytes.Buffer
		if err := provision.Exec(c, "root", provision.LogsCommand(s.web, provision.Vars{Domain: hosted[0].domain()}, "8.4"), &logs); err != nil || !strings.Contains(logs.String(), "journalctl -u") {
			t.Errorf("%s logs: %v\n%s", s.name, err, logs.String())
		}
		for _, a := range hosted {
			if !contains(a.steps, "backup") {
				continue
			}
			var out bytes.Buffer
			if err := provision.Exec(c, "root", provision.BackupNowCommand(a.name), &out); err != nil || !strings.Contains(out.String(), "backup ok") {
				t.Errorf("%s backup now: %v\n%s", a.name, err, out.String())
			}
			ls, _ := c.ExecuteCommand("ls /var/www/versa/" + a.name + "/backups | wc -l")
			t.Logf("%s: %s backup file(s) after backup-now", a.name, strings.TrimSpace(ls))
		}
	}
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	if len(l) > 120 {
		l = l[:120]
	}
	return l
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
