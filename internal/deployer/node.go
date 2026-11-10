package deployer

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/versaDeploy/internal/provision"
	"github.com/user/versaDeploy/internal/ssh"
)

// nodeDepsScript installs the release's node_modules on the server with the user's
// command, unless reuseDependencies already hardlinked them from the previous release.
// pathPrefix puts the project's own node/npm first (provision.ProjectPATH).
func nodeDepsScript(cmd, appDir, pathPrefix string) string {
	return fmt.Sprintf(`cd %s || exit 1
%strue
if [ -d node_modules ]; then echo "reused"; exit 0; fi
[ -f package.json ] || { echo "no package.json in the release"; exit 1; }
%s
rc=$?; [ $rc = 127 ] && echo "command not found: is Node.js installed on the server? (Services → Node.js)"; exit $rc
`, q(appDir), pathPrefix, cmd)
}

// ensureNodeDeps makes sure the new release has node_modules when server_install is set.
func (d *Deployer) ensureNodeDeps(c *ssh.Client, finalDir string) error {
	fe := d.env.Builds.Frontend
	if !fe.Enabled || fe.ServerInstall == "" {
		return nil
	}
	appDir := filepath.ToSlash(filepath.Join(finalDir, "app", fe.ProjectRoot))
	d.log.Info("Installing Node dependencies on the server...")
	timeout := time.Duration(d.env.DeployTimeout) * time.Second
	if timeout < 10*time.Minute {
		timeout = 10 * time.Minute
	}
	out, err := c.ExecuteCommandWithTimeout(shCmd(nodeDepsScript(fe.ServerInstall, appDir, provision.ProjectPATH(d.env.RemotePath))), timeout)
	out = strings.TrimSpace(out)
	if err != nil {
		return fmt.Errorf("server_install failed: %w\n%s", err, out)
	}
	if out == "reused" {
		d.log.Info("  Reused node_modules from the previous release")
	} else {
		d.log.Debug("%s", out)
		d.log.Info("  node_modules ready")
	}
	return nil
}
