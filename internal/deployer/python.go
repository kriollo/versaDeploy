package deployer

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/ssh"
)

// venvScript creates the Python virtualenv of a release on the server and installs the
// dependencies into it, unless one was already carried over from the previous release
// (reuseDependencies hardlinks it when the requirements didn't change). A venv built
// on the developer's machine wouldn't work there: other OS, paths and native wheels.
func venvScript(py config.PythonBuildConfig, appDir string) string {
	venv := filepath.ToSlash(py.VenvPath)
	pyBin := venv + "/bin/python"
	pyCmd := py.PythonCommand
	if pyCmd == "" {
		pyCmd = "python3"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `cd %s || exit 1
if [ -x %[2]s ]; then echo "reused"; exit 0; fi
if ! command -v %[3]s >/dev/null 2>&1; then echo "%[3]s not found on the server (set python.python_command)"; exit 1; fi
rm -rf %[4]s
%[3]s -m venv %[4]s || { echo "could not create the virtualenv: install Python's venv module on the server (Debian/Ubuntu: apt install python3-venv)"; exit 1; }
echo "created %[4]s"
`, q(appDir), q(pyBin), q(pyCmd), q(venv))

	switch py.PackageManager {
	case "poetry":
		// VIRTUAL_ENV makes poetry install into our venv instead of its own
		args := "install --no-root --no-interaction"
		if !py.InstallDevDeps {
			args += " --without dev"
		}
		fmt.Fprintf(&b, "command -v poetry >/dev/null 2>&1 || { echo \"poetry not found on the server\"; exit 1; }\nVIRTUAL_ENV=\"$PWD\"/%s PATH=\"$PWD\"/%s/bin:$PATH poetry %s || exit 1\n", q(venv), q(venv), args)
	case "pipenv":
		args := "install --deploy"
		if py.InstallDevDeps {
			args += " --dev"
		}
		fmt.Fprintf(&b, "command -v pipenv >/dev/null 2>&1 || { echo \"pipenv not found on the server\"; exit 1; }\nVIRTUAL_ENV=\"$PWD\"/%s PATH=\"$PWD\"/%s/bin:$PATH pipenv %s || exit 1\n", q(venv), q(venv), args)
	default:
		var index []string
		switch {
		case py.TorchIndex != "":
			index = []string{"--extra-index-url", q(py.TorchIndex)}
		case py.PyPIMirror != "":
			index = []string{"-i", q(py.PyPIMirror)}
		}
		cache := "--no-cache-dir"
		if py.UseCache {
			cache = ""
		}
		for _, req := range append([]string{py.RequirementsFile}, py.ExtraRequirements...) {
			args := strings.Join(append([]string{"install", "-r", q(req), cache}, index...), " ")
			fmt.Fprintf(&b, "if [ -f %[1]s ]; then %[2]s -m pip %[3]s || exit 1; echo \"installed %[4]s\"; fi\n", q(req), q(pyBin), args, strings.ReplaceAll(req, `"`, ""))
		}
	}
	return b.String()
}

// ensurePythonVenv makes sure the new release has a working virtualenv.
func (d *Deployer) ensurePythonVenv(c *ssh.Client, finalDir string) error {
	py := d.env.Builds.Python
	if !py.Enabled || py.BuildBinary {
		return nil // a PyInstaller binary carries its own dependencies
	}
	appDir := filepath.ToSlash(filepath.Join(finalDir, "app", py.ProjectRoot))
	d.log.Info("Preparing Python virtualenv on the server...")
	timeout := time.Duration(d.env.DeployTimeout) * time.Second
	if timeout < 10*time.Minute {
		timeout = 10 * time.Minute // big wheels (torch) take a while
	}
	out, err := c.ExecuteCommandWithTimeout(shCmd(venvScript(py, appDir)), timeout)
	out = strings.TrimSpace(out)
	if err != nil {
		return fmt.Errorf("failed to prepare the Python virtualenv: %w\n%s", err, out)
	}
	if out == "reused" {
		d.log.Info("  Reused virtualenv from the previous release")
	} else {
		d.log.Debug("%s", out)
		d.log.Info("  Virtualenv ready")
	}
	return nil
}
