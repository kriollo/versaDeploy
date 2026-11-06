package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/config"
)

// A server-chosen name inserted by tab completion must stay one literal word.
func TestShellEscapeKeepsRemoteNamesLiteral(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	dir := t.TempDir()
	name := "y$(touch PWNED); `touch PWNED2` 'q' \"d\" &|<>*?~!#"
	if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", "cd "+shellEscape(name)+" && basename \"$PWD\"")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cd into escaped name failed: %v %s", err, out)
	}
	if got := string(out); got != name+"\n" {
		t.Fatalf("resolved %q, want %q", got, name)
	}
	for _, f := range []string{"PWNED", "PWNED2"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Fatal("name was executed")
		}
	}
	if got := shellEscape("app/releases-1.2_x"); got != "app/releases-1.2_x" {
		t.Errorf("plain names must stay readable, got %q", got)
	}
}

func TestRsyncVersionQuoting(t *testing.T) {
	for out, old := range map[string]bool{
		"rsync  version 3.2.7  protocol version 31":  false,
		"rsync  version 3.2.4  protocol version 31":  false,
		"rsync  version 3.2.3  protocol version 31":  true,
		"rsync  version 3.1.2  protocol version 31":  true,
		"rsync  version 2.6.8  protocol version 29":  true,
		"rsync  version v3.4.1  protocol version 32": false,
	} {
		if got := rsyncBefore324(out); got != old {
			t.Errorf("%q: old=%v, want %v", out, got, old)
		}
	}
	args := strings.Join(buildRsyncArgs(&config.SSHConfig{Port: 22, KnownHostsFile: "/k h", StrictHostKey: true}, []string{"a"}, "b"), " ")
	if !strings.Contains(args, "UserKnownHostsFile='/k h'") || !strings.Contains(args, "StrictHostKeyChecking=yes") {
		t.Errorf("rsync ssh misses the host-key policy: %s", args)
	}
}

// The prompt must fit in the pane: the content pane clips what is taller.
func TestTerminalPromptFitsPane(t *testing.T) {
	for _, h := range []int{20, 37, 50} {
		term := newTerminalModel()
		term.initViewport(100, h)
		out := term.view(100, h)
		lines := strings.Split(out, "\n")
		if len(lines) > h {
			t.Fatalf("height %d: view has %d lines, the prompt is clipped", h, len(lines))
		}
		if !strings.Contains(out, "$") {
			t.Fatalf("height %d: no prompt in the view", h)
		}
	}
}
