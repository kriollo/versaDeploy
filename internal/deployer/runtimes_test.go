package deployer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The project's bin dir links its php and every node tool, follows a version change and
// drops links versa no longer wants (a stale node major, anything else).
func TestBinLinkScript(t *testing.T) {
	requireRealSymlinks(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, ".versa/bin")
	php81 := filepath.Join(dir, "php8.1")
	php84 := filepath.Join(dir, "php8.4")
	node20 := filepath.Join(dir, "node-20/bin")
	os.MkdirAll(node20, 0o755)
	for _, f := range []string{php81, php84, node20 + "/node", node20 + "/npm"} {
		os.WriteFile(f, []byte("#!/bin/sh\n"), 0o755)
	}
	run := func(php, node string) string {
		t.Helper()
		out, err := exec.Command("sh", "-c", binLinkScript(bin, php, "", node, "20")).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return string(out)
	}
	target := func(name string) string { l, _ := os.Readlink(filepath.Join(bin, name)); return l }

	run(php81, node20)
	os.Symlink("/etc/passwd", filepath.Join(bin, "stray"))
	if target("php") != php81 || target("node") != node20+"/node" || target("npm") != node20+"/npm" {
		t.Fatalf("links: php=%q node=%q npm=%q", target("php"), target("node"), target("npm"))
	}
	run(php84, "")
	if target("php") != php84 || target("node") != "" || target("stray") != "" {
		t.Fatalf("after the change: php=%q node=%q stray=%q", target("php"), target("node"), target("stray"))
	}
	if out := run(filepath.Join(dir, "php7.4"), ""); out != "missing php "+filepath.Join(dir, "php7.4")+"\n" {
		t.Fatalf("a missing PHP must be reported: %q", out)
	}
}
