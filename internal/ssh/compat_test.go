package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDfAvailable(t *testing.T) {
	cases := map[string]int64{
		// df -Pk on RHEL5 with a long LVM device name: stays on one line
		"Filesystem         1024-blocks      Used Available Capacity Mounted on\n" +
			"/dev/mapper/VolGroup00-LogVol00  25396228  12698114  12698114      50% /\n": 12698114 * 1024,
		// modern coreutils
		"Filesystem     1024-blocks   Used Available Capacity Mounted on\n/dev/sda1 1000 400 600 40% /var\n": 600 * 1024,
	}
	for out, want := range cases {
		got, err := parseDfAvailable(out)
		if err != nil || got != want {
			t.Errorf("parseDfAvailable(%q) = %d, %v; want %d", out, got, err, want)
		}
	}

	// Wrapped (non -P) output must be rejected, never read "50%" as 50 bytes
	wrapped := "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/mapper/VolGroup00-LogVol00\n 25396228 12698114 12698114 50% /\n"
	if got, err := parseDfAvailable(wrapped); err == nil {
		t.Errorf("wrapped df output accepted as %d", got)
	}
}

func TestShWrapQuoting(t *testing.T) {
	cmd := `echo "a b" 'it'"'"'s' $((1+1)) | tr -d x`
	direct, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		t.Skip("no /bin/sh")
	}
	wrapped, err := exec.Command("/bin/sh", "-c", shWrap(cmd)).Output()
	if err != nil || string(wrapped) != string(direct) {
		t.Errorf("shWrap changed behavior: %q vs %q (%v)", wrapped, direct, err)
	}
}

// TestSymlinkSwap runs the swap against a "current" link that already points to a
// directory (mv without -T would move the tmp link *into* it), including with mv -T,
// perl and python all unavailable to exercise the rm + mv fallback.
func TestSymlinkSwap(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	requireRealSymlinks(t)
	stub := func(dir, name, body string) {
		os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0755)
	}
	noTools := t.TempDir()
	stub(noTools, "mv", `case "$1" in -T*) exit 1;; esac; exec /bin/mv "$@"`)
	for _, n := range []string{"perl", "python3", "python"} {
		stub(noTools, n, "exit 1")
	}

	cases := map[string][]string{"default": {"sh", "-c"}, "fallback": {"sh", "-c"}}
	if _, err := exec.LookPath("busybox"); err == nil {
		cases["busybox"] = []string{"busybox", "sh", "-c"}
	}
	for name, shell := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			os.MkdirAll(filepath.Join(dir, "releases", "a"), 0755)
			os.MkdirAll(filepath.Join(dir, "releases", "b dir"), 0755)
			link := filepath.Join(dir, "current")
			os.Symlink("releases/a", link)

			cmd := exec.Command(shell[0], append(shell[1:], symlinkSwapCmd("releases/b dir", link))...)
			if name == "fallback" {
				cmd.Env = append(os.Environ(), "PATH="+noTools+":"+os.Getenv("PATH"))
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("swap failed: %v: %s", err, out)
			}
			if got, _ := os.Readlink(link); got != "releases/b dir" || strings.TrimSpace(string(out)) != got {
				t.Errorf("current -> %q (output %q), want releases/b dir", got, out)
			}
			if _, err := os.Lstat(filepath.Join(dir, "releases", "a", "current.tmp")); err == nil {
				t.Error("tmp link was moved into the old release")
			}
		})
	}
}

// requireRealSymlinks skips the test when this machine can't create real symlinks
// from Go and from sh (Windows without Developer Mode, where Git Bash's ln -s copies
// instead). The scripts under test only ever run on the Linux server.
func requireRealSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink("d", filepath.Join(dir, "go-link")); err != nil {
		t.Skipf("symlinks not supported here: %v", err)
	}
	cmd := exec.Command("sh", "-c", "mkdir d && ln -s d sh-link")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("sh can't create symlinks: %v: %s", err, out)
	}
	if _, err := os.Readlink(filepath.Join(dir, "sh-link")); err != nil {
		t.Skip("sh's ln -s doesn't create real symlinks here (Git Bash without Developer Mode)")
	}
}
