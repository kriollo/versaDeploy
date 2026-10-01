package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"testing"
)

// Runs the real stats script under the local /bin/sh: every key must produce a value.
func TestServerStatsScript(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs /proc")
	}
	out, err := exec.Command("/bin/sh", "-c", fmt.Sprintf(serverStatsScript, `"/"`)).Output()
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	stats := parseStats(string(out))
	for _, k := range []string{"disk", "ram", "load", "uptime", "os", "cpu"} {
		if stats[k] == "" {
			t.Errorf("%s is empty; output:\n%s", k, out)
		}
	}
}
