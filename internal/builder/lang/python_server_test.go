package lang

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
)

// `exec FLASK_APP=x python` looks for a program named FLASK_APP=x: the app never started.
func TestRunServerFlaskStarts(t *testing.T) {
	s := runServerScript(config.PythonBuildConfig{VenvPath: "venv", EntryPoint: "app.py", WebFramework: "flask", WebHost: "127.0.0.1", WebPort: 8000}, "")
	if !strings.Contains(s, "exec env FLASK_APP=app.py ") {
		t.Fatalf("script:\n%s", s)
	}
	if out, err := exec.Command("sh", "-c", "exec env FLASK_APP=app.py true").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
