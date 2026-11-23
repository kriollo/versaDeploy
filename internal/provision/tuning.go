package provision

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/versaDeploy/internal/config"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

// Tuned is what provision.tuning resolves to on one server. The zero value means "off":
// everything renders as it did before tuning existed.
type Tuned struct {
	Active                   bool
	Profile                  string
	CPUs, MemMB              int
	NumThreads, MaxThreads   int // FrankenPHP
	MaxWait                  string
	FPMChildren, FPMStart    int
	FPMMinSpare, FPMMaxSpare int
	SharedBuffersMB          int // PostgreSQL (server_wide)
	EffectiveCacheMB         int
	MaintenanceMB            int
	InnoDBMB                 int // MariaDB (server_wide)
	ServerWide               bool
}

const phpWorkerMB = 80 // ponytail: estimate per PHP worker; measure the app's RSS before raising it

var (
	shareRe     = regexp.MustCompile(`^([1-9][0-9]?|100)%$`)
	uploadRe    = regexp.MustCompile(`^[1-9][0-9]*[MG]$`)
	maxWaitRe   = regexp.MustCompile(`^[1-9][0-9]*(ms|s)$`)
	timezoneRe  = regexp.MustCompile(`^[A-Za-z_]+(/[A-Za-z0-9_+-]+)*$`)
	backupAtRe  = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	restoreFile = regexp.MustCompile(`^[A-Za-z0-9_]+-[0-9]{8}-[0-9]{6}\.sql\.gz$`)
)

// ComputeTuning sizes the stack for the server: cpus and memMB come from detection.
func ComputeTuning(t config.TuningConfig, cpus, memMB int) Tuned {
	if t.Profile == "" || t.Profile == "off" || cpus < 1 || memMB < 1 {
		return Tuned{}
	}
	share := 100
	if m := shareRe.FindStringSubmatch(t.Share); m != nil {
		share, _ = strconv.Atoi(m[1])
	}
	app := memMB * share / 100
	factor := 2
	if t.Profile == "event" {
		factor = 4
	}
	threads := max(2, min(factor*cpus, app*40/100/phpWorkerMB))
	if t.PHP.MaxThreads > 0 {
		threads = t.PHP.MaxThreads
	}
	wait := t.PHP.MaxWait
	if wait == "" {
		wait = "15s"
	}
	return Tuned{
		Active: true, Profile: t.Profile, CPUs: cpus, MemMB: memMB,
		NumThreads: max(1, min(cpus, threads)), MaxThreads: threads, MaxWait: wait,
		FPMChildren: threads, FPMStart: max(1, threads/4), FPMMinSpare: max(1, threads/4), FPMMaxSpare: max(2, threads/2),
		SharedBuffersMB: app * 15 / 100, EffectiveCacheMB: app / 2, MaintenanceMB: min(1024, app*5/100),
		InnoDBMB: app * 30 / 100, ServerWide: t.Database.ServerWide,
	}
}

func validTuning(t config.TuningConfig) error {
	switch {
	case t.Profile != "" && t.Profile != "off" && t.Profile != "auto" && t.Profile != "small" && t.Profile != "event":
		return fmt.Errorf("provision.tuning.profile must be off, auto, small or event (got %q)", t.Profile)
	case t.Share != "" && !shareRe.MatchString(t.Share):
		return fmt.Errorf("provision.tuning.share must look like 50%% (got %q)", t.Share)
	case t.UploadMax != "" && !uploadRe.MatchString(t.UploadMax):
		return fmt.Errorf("provision.tuning.upload_max must look like 200M (got %q)", t.UploadMax)
	case t.PHP.MaxThreads < 0 || t.PHP.MaxThreads > 4096:
		return fmt.Errorf("provision.tuning.php.max_threads out of range (got %d)", t.PHP.MaxThreads)
	case t.PHP.MaxWait != "" && !maxWaitRe.MatchString(t.PHP.MaxWait):
		return fmt.Errorf("provision.tuning.php.max_wait must look like 15s (got %q)", t.PHP.MaxWait)
	}
	return nil
}

// UploadMax is the request size nginx, Apache and PHP accept: one value, so none of them
// takes a file another one then throws away.
func (v Vars) UploadMax() string {
	if v.Tuning.UploadMax != "" {
		return v.Tuning.UploadMax
	}
	return "200M"
}

// UploadBytes is UploadMax for Apache's LimitRequestBody.
func (v Vars) UploadBytes() int64 {
	s := v.UploadMax()
	n, _ := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if strings.HasSuffix(s, "G") {
		return n << 30
	}
	return n << 20
}

// PHPTimezone is PHP's date.timezone.
func (v Vars) PHPTimezone() string {
	if v.Timezone != "" {
		return v.Timezone
	}
	return "America/Santiago"
}

// HSTSValue is the header's default. includeSubDomains and preload can break subdomains
// outside this app, so they are opt-in.
func (v Vars) HSTSValue() string {
	if v.HSTSPreload {
		return "max-age=31536000; includeSubDomains; preload"
	}
	return "max-age=31536000"
}

// HSTSVar is the nginx variable carrying it.
func (v Vars) HSTSVar() string {
	if v.HSTSPreload {
		return "$versa_hsts_preload"
	}
	return "$versa_hsts"
}

// FPMSocket is the socket of the vhost: a tuned project gets its own pool, so its
// limits never change the other apps on the same PHP version.
func (v Vars) FPMSocket() string {
	if v.Tune.Active {
		return "/run/php/php" + v.PHPVersion + "-fpm-" + nginxIdent.ReplaceAllString(v.Project, "_") + ".sock"
	}
	return "/run/php/php" + v.PHPVersion + "-fpm.sock"
}

// Upstream names the keepalive upstream of a tuned proxied app ("" when off).
func (v Vars) Upstream() string {
	if !v.Tune.Active || v.ProxyPort == 0 {
		return ""
	}
	return "versa_app_" + nginxIdent.ReplaceAllString(v.Project, "_")
}

// TunePlan describes what `versa tune` would do, by how it takes effect.
func TunePlan(v Vars) []string {
	t := v.Tune
	if !t.Active {
		return nil
	}
	out := []string{fmt.Sprintf("server: %d vCPU, %d MB · profile %s", t.CPUs, t.MemMB, t.Profile)}
	if v.WebServer == "nginx" {
		out = append(out, fmt.Sprintf("[reload]   nginx: upload %s, keepalive to the app", v.UploadMax()))
	}
	if v.FrankenPHP {
		out = append(out, fmt.Sprintf("[restart]  FrankenPHP (%s): %d-%d threads, wait %s", v.AppService, t.NumThreads, t.MaxThreads, t.MaxWait))
	} else if v.ProxyPort == 0 && v.PHPVersion != "" {
		out = append(out, fmt.Sprintf("[reload]   PHP-FPM: own pool, up to %d workers", t.FPMChildren))
	}
	switch v.Database {
	case "postgres":
		out = append(out, "[live]     PostgreSQL role "+v.DBUser+": idle transactions 60s, log queries over 500ms")
		if t.ServerWide {
			out = append(out, fmt.Sprintf("[others]   PostgreSQL (all apps): effective_cache_size %dMB, maintenance_work_mem %dMB", t.EffectiveCacheMB, t.MaintenanceMB),
				fmt.Sprintf("[restart]  PostgreSQL (all apps): shared_buffers %dMB", t.SharedBuffersMB))
		}
	case "mariadb":
		if t.ServerWide {
			out = append(out, fmt.Sprintf("[others]   MariaDB (all apps): innodb_buffer_pool_size %dMB", t.InnoDBMB))
		}
	}
	return out
}

// TuneBundle applies provision.tuning. Only the services named in restart are restarted.
func TuneBundle(v Vars, restart []string) (Bundle, error) {
	if !v.Tune.Active {
		return nil, fmt.Errorf("provision.tuning.profile is off: nothing to apply")
	}
	if v.AdminEmail == "" && v.Domain != "" {
		v.AdminEmail = "admin@" + v.Domain
	}
	allowed := map[string]bool{"postgresql": true, "mariadb": true}
	if v.AppService != "" {
		allowed[v.AppService] = true
	}
	for _, r := range restart {
		if !allowed[r] {
			return nil, fmt.Errorf("--restart %q: use postgresql, mariadb or the app's service (%s)", r, v.AppService)
		}
	}
	if err := v.Validate([]string{"dirs"}); err != nil {
		return nil, err
	}
	common, _ := assets.ReadFile("scripts/common.sh")
	body, _ := assets.ReadFile("scripts/tune.sh")
	b := Bundle{"env": []byte(v.env() + "TUNE_RESTART=" + versassh.ShellQuote(strings.Join(restart, " ")) + "\nAPP_SERVICE=" + versassh.ShellQuote(v.AppService) + "\n")}
	b["run.sh"] = append(append(common, []byte("\nsection 'Tuning'\nstep_tune() {\n")...), append(body, []byte("\n}\nstep_tune\nok 'Tuning applied'\n")...)...)
	if v.WebServer != "" {
		for _, ssl := range []bool{false, true} {
			out, err := RenderVhost(v, ssl)
			if err != nil {
				return nil, err
			}
			name := "vhost.conf"
			if ssl {
				name = "vhost-ssl.conf"
			}
			b[name] = []byte(out)
		}
	}
	if v.FrankenPHP {
		c, err := renderCaddyfile(v)
		if err != nil {
			return nil, err
		}
		b["Caddyfile"] = c
	}
	return b, nil
}

// RestoreScript restores a backup into a throwaway database and compares row counts. The
// live database is never touched. The dump is written by the deploy user, so its SQL runs
// as that user's unprivileged role: as postgres or root it could execute anything.
func RestoreScript(v Vars, file string, keep bool) (string, error) {
	if !identRe.MatchString(v.DBName) || !nameRe.MatchString(v.DeployUser) {
		return "", fmt.Errorf("db_name and deploy_user are required")
	}
	if v.Database != "postgres" && v.Database != "mariadb" {
		return "", fmt.Errorf("restore needs provision.database: postgres or mariadb")
	}
	q := versassh.ShellQuote
	dir := strings.TrimRight(v.RemotePath, "/") + "/backups"
	if file == "" {
		return "ls -1t " + q(dir) + " 2>/dev/null | grep -E '^" + v.DBName + "-[0-9]{8}-[0-9]{6}\\.sql\\.gz$' || echo 'no backups'", nil
	}
	if !restoreFile.MatchString(file) || !strings.HasPrefix(file, v.DBName+"-") {
		return "", fmt.Errorf("%q is not a backup of %s (versa db restore ENV lists them)", file, v.DBName)
	}
	body, _ := assets.ReadFile("scripts/restore.sh")
	return fmt.Sprintf("DATABASE=%s DB_NAME=%s OWNER=%s FILE=%s KEEP=%s\n%s", q(v.Database), q(v.DBName), q(v.DeployUser),
		q(dir+"/"+file), strconv.FormatBool(keep), body), nil
}
