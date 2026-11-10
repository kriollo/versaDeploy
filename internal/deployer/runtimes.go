package deployer

import (
	"fmt"
	"strings"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/provision"
	"github.com/user/versaDeploy/internal/ssh"
)

// projectBinScript points <remote_path>/.versa/bin at the project's own PHP and Node
// (provision.php_version / frankenphp / node_major), so several projects on one server
// each run their version: hooks, cron, services and versa exec put it first in PATH.
// Links are replaced in place (a cron job never finds php missing) and links versa
// doesn't want anymore are removed. "" when the config names no runtime.
func projectBinScript(env *config.Environment) string {
	p := env.Provision
	php, phpv := "", ""
	switch {
	case p.FrankenPHP:
		php = provision.FrankenPHPCLI
	case p.PHPVersion != "":
		php, phpv = "/usr/bin/php"+p.PHPVersion, p.PHPVersion // Debian/Ubuntu; RHEL has only /usr/bin/php
	}
	node := ""
	if p.NodeMajor != "" {
		node = provision.NodeDir(p.NodeMajor) + "/bin"
	}
	if php == "" && node == "" {
		return ""
	}
	return binLinkScript(provision.ProjectBinDir(env.RemotePath), php, phpv, node, p.NodeMajor)
}

// binLinkScript links php (when phpv's PHP isn't already the server's) and every file of
// the node bin dir into bin, dropping links that point anywhere else.
func binLinkScript(bin, php, phpv, node, major string) string {
	return fmt.Sprintf(`b=%s; php=%s; phpv=%s; node=%s; major=%s
mkdir -p "$b" || exit 1
if [ -n "$php" ]; then
	if [ -x "$php" ]; then ln -sfn "$php" "$b/php"
	elif [ -n "$phpv" ] && [ "$(php -r 'echo PHP_MAJOR_VERSION, ".", PHP_MINOR_VERSION;' 2>/dev/null)" = "$phpv" ]; then : # RHEL: the server's only PHP
	elif [ -z "$phpv" ] && grep -qs frankenphp "$(command -v php)"; then : # FrankenPHP's older /usr/local/bin/php
	else echo "missing php $php"; fi
fi
if [ -n "$node" ]; then
	if [ -d "$node" ]; then
		for f in "$node"/*; do [ -e "$f" ] && ln -sfn "$f" "$b/$(basename "$f")"; done
	elif [ "$(node -v 2>/dev/null | sed 's/^v//; s/\..*//')" != "$major" ]; then
		echo "missing node $major"
	fi
fi
for f in "$b"/*; do
	[ -L "$f" ] || continue
	t=$(readlink "$f")
	case "$t" in
	"$php") [ -n "$php" ] || rm -f "$f" ;;
	"$node"/*) [ -n "$node" ] && [ -d "$node" ] || rm -f "$f" ;;
	*) rm -f "$f" ;;
	esac
done
true
`, q(bin), q(php), q(phpv), q(node), q(major))
}

// linkProjectRuntimes runs projectBinScript, warning when a runtime isn't installed.
func (d *Deployer) linkProjectRuntimes(c *ssh.Client) error {
	script := projectBinScript(d.env)
	if script == "" {
		return nil
	}
	out, err := c.ExecuteCommand(shCmd(script))
	if err != nil {
		return fmt.Errorf("linking the project's php/node into %s: %w (%s)", provision.ProjectBinDir(d.env.RemotePath), err, strings.TrimSpace(out))
	}
	for _, l := range strings.Split(out, "\n") {
		if what, ok := strings.CutPrefix(strings.TrimSpace(l), "missing "); ok {
			d.log.Warn("%s is not installed on the server: hooks, cron and services use the server's default (versa → Services → install it, or versa provision %s)", what, d.envName)
		}
	}
	return nil
}
