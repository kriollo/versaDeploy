package provision

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The DB password must reach mysql/psql through stdin, never argv: argv is
// readable by every local user through /proc/<pid>/cmdline.
func TestDBPasswordNotInArgv(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs bash")
	}
	const secret = "S3cret'Marker\\42"
	for _, file := range []string{"mariadb.sh", "postgres.sh"} {
		body, _ := assets.ReadFile("scripts/" + file)
		s := string(body)
		start := strings.Index(s, "pass_sql=")
		end := strings.Index(s[start:], "\nfi\n")
		if start < 0 || end < 0 {
			t.Fatalf("%s: password section not found", file)
		}
		section := s[start : start+end+4]

		for _, exists := range []string{"0", "1"} {
			log := filepath.Join(t.TempDir(), "log")
			script := `set -euo pipefail
die() { echo "$*"; exit 1; }; ok() { :; }; warn() { :; }
client() { printf 'ARGV:%s\n' "$*" >> "$LOG"; [ -t 0 ] || sed 's/^/STDIN:/' >> "$LOG"; case "$*" in *SELECT*) echo "$EXISTS";; esac; }
mysql() { client "$@"; }
sudo() { shift 2; client "$@"; }
` + section
			cmd := exec.Command(bash, "-c", script)
			cmd.Env = append(os.Environ(), "LOG="+log, "EXISTS="+exists, "RESET_DB_PASSWORD=true",
				"DB_USER=appuser", "DB_NAME=appdb", "DB_PASS="+secret)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v\n%s", file, err, out)
			}
			got, _ := os.ReadFile(log)
			sawStdin := false
			for _, line := range strings.Split(string(got), "\n") {
				if strings.HasPrefix(line, "ARGV:") && strings.Contains(line, "Marker") {
					t.Fatalf("%s: password in argv: %s", file, line)
				}
				sawStdin = sawStdin || strings.HasPrefix(line, "STDIN:") && strings.Contains(line, "Marker")
			}
			if !sawStdin {
				t.Fatalf("%s (exists=%s): password never sent on stdin:\n%s", file, exists, got)
			}
		}
	}
}
