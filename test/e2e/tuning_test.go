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

// provision.tuning on PHP-FPM with each web server and database: the project's own pool,
// the database sized, the site still answering, and a backup that really restores.
func TestTuning(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	run(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	ctx := filepath.Join(dir, "ctx")
	os.MkdirAll(ctx, 0o755)
	run(t, "cp", key+".pub", filepath.Join(ctx, "id.pub"))
	versa := filepath.Join(dir, "versa")
	run(t, "go", "build", "-o", versa, "../../cmd/versa")

	for _, c := range []struct {
		name, base, dockerfile, web, php, db, fpmUnit string
		ssh, http                                     int
	}{
		{"ubuntu", "ubuntu:24.04", "Dockerfile", "nginx", "8.4", "postgres", "php8.4-fpm", 12250, 18250},
		{"debian", "debian:12", "Dockerfile", "apache", "8.4", "mariadb", "php8.4-fpm", 12251, 18251},
		{"rocky", "rockylinux:9", "Dockerfile.rhel", "nginx", "8.3", "postgres", "php-fpm", 12252, 18252},
	} {
		t.Run(c.name, func(t *testing.T) {
			img := "versa-e2e-tune-" + c.name
			df, _ := filepath.Abs(c.dockerfile)
			run(t, "docker", "build", "-q", "-t", img, "--build-arg", "BASE="+c.base, "-f", df, ctx)
			exec.Command("docker", "rm", "-f", img).Run()
			run(t, "docker", "run", "-d", "--name", img, "--privileged", "--cgroupns=host",
				"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock",
				"-p", fmt.Sprintf("127.0.0.1:%d:22", c.ssh), "-p", fmt.Sprintf("127.0.0.1:%d:80", c.http), img)
			if os.Getenv("VERSA_E2E_KEEP") == "" {
				t.Cleanup(func() { exec.Command("docker", "rm", "-f", img).Run() })
			}
			known := filepath.Join(dir, c.name+"_known_hosts")
			var keys []byte
			for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline) && len(keys) == 0; time.Sleep(2 * time.Second) {
				keys, _ = exec.Command("ssh-keyscan", "-p", fmt.Sprint(c.ssh), "127.0.0.1").Output()
			}
			os.WriteFile(known, keys, 0o600)
			root := func(cmd string) string {
				out, _ := exec.Command("ssh", "-p", fmt.Sprint(c.ssh), "-i", key, "-o", "UserKnownHostsFile="+known,
					"-o", "BatchMode=yes", "root@127.0.0.1", cmd).CombinedOutput()
				return strings.TrimSpace(string(out))
			}
			webUnit := map[string]string{"nginx": "nginx", "apache": "apache2"}[c.web]
			src := filepath.Join(dir, "src-"+c.name)
			run(t, "cp", "-a", filepath.Join("apps", "php"), src)
			os.WriteFile(filepath.Join(src, "deploy.yml"), []byte(fmt.Sprintf(`project: tn
environments:
  vps:
    ssh: { host: 127.0.0.1, port: %d, user: deploy, key_path: %s, known_hosts_file: %s, strict_host_key: true }
    remote_path: /var/www/tn
    provision:
      ssh: { user: root }
      domain: tn.localhost
      web_server: %s
      php_version: "%s"
      database: %s
      db_name: tn
      db_user: tn_user
      db_password: e2e-secret
      deploy_user: deploy
      backup: true
      backup_time: "04:30"
      timezone: America/Santiago
      tuning:
        profile: event
        share: 10%%
        upload_max: 150M
        database: { server_wide: true }
    ignored_paths: [".git"]
    builds:
      php: { enabled: true, root: '.', composer_command: 'composer install --no-dev' }
    services_reload: ['sudo systemctl reload %s', 'sudo systemctl reload %s']
    health_check: { url: "http://tn.localhost:%d/" }
`, c.ssh, key, known, c.web, c.php, c.db, c.fpmUnit, webUnit, c.http)), 0o644)
			repo := gitRepo(t, src)
			versaIn := func(stdin string, args ...string) string {
				t.Helper()
				cmd := exec.Command(versa, args...)
				cmd.Dir, cmd.Stdin = repo, strings.NewReader(stdin)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("versa %v: %v\n%s", args, err, out)
				}
				return string(out)
			}
			versaIn("", "provision", "vps", "-y")
			versaIn("y\n", "deploy", "vps", "--initial-deploy", "--no-gui")
			if plan := versaIn("", "tune", "vps"); !strings.Contains(plan, "own pool") {
				t.Errorf("plan:\n%s", plan)
			}
			dbUnit := map[string]string{"postgres": "postgresql", "mariadb": "mariadb"}[c.db]
			versaIn("", "tune", "vps", "--apply", "--restart", dbUnit)

			sock := "/run/php/php" + c.php + "-fpm-tn.sock"
			if root("test -S "+sock+" && echo ok") != "ok" {
				t.Errorf("no project pool socket: %s", root("systemctl status "+c.fpmUnit+" --no-pager -n 12"))
			}
			if body := fetch(t, c.http, "tn.localhost", "/"); !strings.Contains(body, "versa-e2e php") {
				t.Errorf("site through the project's pool answers %q", firstLine(body))
			}
			if got := root("ls /etc/logrotate.d/versa-tn"); !strings.HasSuffix(got, "versa-tn") {
				t.Errorf("logrotate: %s", got)
			}
			if c.db == "postgres" {
				if root(`cd /tmp && sudo -u postgres psql -tAc "SELECT setting FROM pg_settings WHERE name='shared_buffers'"`) == "16384" {
					t.Error("shared_buffers wasn't applied")
				}
				if !strings.Contains(root(`cd /tmp && sudo -u postgres psql -tAc "SELECT rolconfig FROM pg_roles WHERE rolname='tn_user'"`), "idle_in_transaction_session_timeout=60s") {
					t.Error("role limits weren't applied")
				}
			} else if root(`mysql -N -e "SELECT @@innodb_buffer_pool_size"`) == "134217728" {
				t.Error("innodb_buffer_pool_size wasn't applied")
			}

			root("systemctl start versa-backup-tn.service")
			list := versaIn("", "db", "restore", "vps")
			var file string
			for _, l := range strings.Fields(strings.ReplaceAll(list, "\r", "")) {
				if strings.HasPrefix(l, "tn-") && strings.HasSuffix(l, ".sql.gz") {
					file = l
					break
				}
			}
			if file == "" {
				t.Fatalf("no backup listed:\n%s", list)
			}
			if out := versaIn("", "db", "restore", "vps", file); !strings.Contains(out, "restore ok") {
				t.Errorf("restore:\n%s", out)
			}
			q := `cd /tmp && sudo -u postgres psql -tAc "SELECT count(*) FROM pg_database WHERE datname LIKE 'tn_restore_%'"`
			if c.db == "mariadb" {
				q = `mysql -N -e "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name LIKE 'tn_restore_%'"`
			}
			if got := root(q); got != "0" {
				t.Errorf("temporary databases left: %s", got)
			}
		})
	}
}
