//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMultiVersion runs several PHP and Node versions on one Ubuntu server, through the
// CLI as a user would (versa provision + versa deploy): PHP-FPM 8.4, then 8.1 and 8.5 next
// to it, FrankenPHP next to them, Node 22 and 20. Each app's web, hooks, cron and services
// run its version; the server's `php` stays the first one (8.4). Also covers a second
// project on a server: its own vhost and sudoers are preselected.
//
//	go test -tags e2e -v -run TestMultiVersion -timeout 30m ./test/e2e/
func TestMultiVersion(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	const sshPort, httpPort = 2240, 8240
	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	run(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	ctx := filepath.Join(dir, "ctx")
	os.MkdirAll(ctx, 0o755)
	run(t, "cp", key+".pub", filepath.Join(ctx, "id.pub"))
	versa := filepath.Join(dir, "versa")
	run(t, "go", "build", "-o", versa, "../../cmd/versa")

	img := "versa-e2e-multi"
	df, _ := filepath.Abs("Dockerfile")
	run(t, "docker", "build", "-q", "-t", img, "-f", df, ctx)
	exec.Command("docker", "rm", "-f", img).Run()
	run(t, "docker", "run", "-d", "--name", img, "--privileged", "--cgroupns=host",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock",
		"-p", fmt.Sprintf("127.0.0.1:%d:22", sshPort), "-p", fmt.Sprintf("127.0.0.1:%d:80", httpPort), img)
	if os.Getenv("VERSA_E2E_KEEP") == "" {
		t.Cleanup(func() { exec.Command("docker", "rm", "-f", img).Run() })
	}
	knownHosts := filepath.Join(dir, "known_hosts")
	var keys []byte
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline) && len(keys) == 0; time.Sleep(2 * time.Second) {
		keys, _ = exec.Command("ssh-keyscan", "-p", fmt.Sprint(sshPort), "127.0.0.1").Output()
	}
	os.WriteFile(knownHosts, keys, 0o600)
	root := func(cmd string) string {
		out, _ := exec.Command("ssh", "-p", fmt.Sprint(sshPort), "-i", key, "-o", "UserKnownHostsFile="+knownHosts,
			"-o", "BatchMode=yes", "root@127.0.0.1", cmd).CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	type multiApp struct{ name, fixture, provision, builds, extra, want string }
	php := func(name, ver string) multiApp {
		return multiApp{name, "php", `php_version: "` + ver + `"`,
			`php: { enabled: true, root: '.', composer_command: 'composer install --no-dev' }`,
			"    services_reload: ['sudo systemctl reload php" + ver + "-fpm', 'sudo systemctl reload nginx']\n", "php " + ver}
	}
	node := func(name, major string, port int) multiApp {
		return multiApp{name, "node", fmt.Sprintf("node_major: %q\n      proxy_port: %d", major, port),
			`frontend: { enabled: true, npm_command: 'npm install', compile_command: 'npm run build' }`,
			fmt.Sprintf("    services_reload: ['sudo systemctl reload nginx']\n    services: [{ name: %s, exec: 'npm start', environment: { PORT: \"%d\" } }]\n", name, port),
			"node v" + major + "."}
	}
	apps := []multiApp{
		php("a84", "8.4"), php("a81", "8.1"), php("a85", "8.5"), // 8.5: Debian would make it the server's php
		{"fr", "php", "frankenphp: true\n      proxy_port: 8000",
			`php: { enabled: true, root: '.', composer_command: 'composer install --no-dev' }`,
			"    services_reload: ['sudo systemctl reload nginx']\n    services:\n      - name: fr\n        exec: /usr/local/bin/frankenphp run --config /etc/frankenphp/fr.Caddyfile\n        reload: curl -fsS --unix-socket /var/www/fr/.frankenphp-opcache.sock http://localhost/\n", "php 8.5"},
		node("n22", "22", 3022), node("n20", "20", 3020),
	}
	repos := map[string]string{}
	for _, a := range apps {
		hook := `php -r 'echo "hook ", PHP_VERSION, PHP_EOL;'`
		cron := `      cron: ["* * * * * php -r 'file_put_contents(\"/tmp/` + a.name + `-cron\", PHP_VERSION);'"]` + "\n"
		if a.fixture == "node" {
			hook, cron = `node -e 'console.log("hook", process.version)'`, ""
		}
		cfg := fmt.Sprintf(`project: %[1]s
environments:
  vps:
    ssh: { host: 127.0.0.1, port: %[2]d, user: deploy, key_path: %[3]s, known_hosts_file: %[4]s, strict_host_key: true }
    remote_path: /var/www/%[1]s
    provision:
      ssh: { user: root }
      domain: %[1]s.localhost
      web_server: nginx
      database: none
      deploy_user: deploy
      %[5]s
%[6]s    ignored_paths: [".git"]
    builds:
      %[7]s
%[8]s    post_deploy: [%[9]q]
    health_check: { url: "http://%[1]s.localhost:%[10]d/" }
`, a.name, sshPort, key, knownHosts, a.provision, cron, a.builds, a.extra, hook, httpPort)
		src := filepath.Join(dir, "src-"+a.name)
		run(t, "cp", "-a", filepath.Join("apps", a.fixture), src)
		os.WriteFile(filepath.Join(src, "deploy.yml"), []byte(cfg), 0o644)
		repos[a.name] = gitRepo(t, src)
	}
	versaIn := func(app, stdin string, args ...string) (string, error) {
		c := exec.Command(versa, args...)
		c.Dir, c.Stdin = repos[app], strings.NewReader(stdin)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	for _, a := range apps {
		if out, err := versaIn(a.name, "", "provision", "vps", "-y"); err != nil {
			t.Fatalf("%s: versa provision: %v\n%s", a.name, err, out)
		}
		out, err := versaIn(a.name, "y\n", "deploy", "vps", "--initial-deploy", "--no-gui")
		if err != nil {
			t.Fatalf("%s: versa deploy: %v\n%s", a.name, err, out)
		}
		// want "php 8.4" → the hook prints "hook 8.4.x"; "node v22." → "hook v22.x"
		if ver := strings.Fields(a.want)[1]; !strings.Contains(out, "hook "+ver) || strings.Contains(out, "Service reload command failed") {
			t.Errorf("%s: the hook didn't run %s, or a reload failed:\n%s", a.name, ver, out)
		}
		if root("php -r 'echo PHP_MAJOR_VERSION, \".\", PHP_MINOR_VERSION;'") != "8.4" {
			t.Fatalf("after %s the server's php is %s, want 8.4 (the first project's)", a.name, root("php -v | head -1"))
		}
	}
	for _, a := range apps {
		if body := fetch(t, httpPort, a.name+".localhost", "/"); !strings.Contains(body, "versa-e2e "+a.want) {
			t.Errorf("%s answers %q, want %q", a.name, firstLine(body), a.want)
		}
	}
	// both majors in /opt/versa; the first one installed is also the server's node
	if out := root("ls /opt/versa; command -v node; node -v | cut -d. -f1"); out != "node-20\nnode-22\n/usr/local/bin/node\nv22" {
		t.Errorf("Node majors / the server's node: %q", out)
	}
	// cron runs each project's php (the cron file's PATH starts with its .versa/bin)
	want := map[string]string{"a84": "8.4", "a81": "8.1", "a85": "8.5", "fr": "8.5"}
	for deadline := time.Now().Add(150 * time.Second); ; time.Sleep(5 * time.Second) {
		got := map[string]string{}
		for name := range want {
			if v := root("cat /tmp/" + name + "-cron 2>/dev/null"); len(v) >= 3 {
				got[name] = v[:3]
			}
		}
		if fmt.Sprint(got) == fmt.Sprint(want) {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("cron php versions = %v, want %v", got, want)
			break
		}
	}
	// a re-run changes nothing
	if out, _ := versaIn("a81", "", "provision", "vps", "--dry-run"); !strings.Contains(out, "Nothing to install") {
		t.Errorf("re-running a81's install would change something:\n%s", out)
	}
}
