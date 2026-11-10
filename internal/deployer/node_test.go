package deployer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNodeDepsScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	app := filepath.Join(dir, "app")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "package.json"), []byte("{}"), 0o644)
	// stub npm: "installs" by creating node_modules
	os.WriteFile(filepath.Join(bin, "npm"), []byte("#!/bin/sh\nmkdir node_modules && echo installed\n"), 0o755)
	run := func(cmd string) (string, error) {
		c := exec.Command("sh", "-c", nodeDepsScript(cmd, app, ""))
		c.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
		out, err := c.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if out, err := run("npm ci --omit=dev"); err != nil || out != "installed" {
		t.Fatalf("first run: %q %v", out, err)
	}
	if out, err := run("npm ci --omit=dev"); err != nil || out != "reused" {
		t.Fatalf("second run must reuse node_modules: %q %v", out, err)
	}
	os.RemoveAll(filepath.Join(app, "node_modules"))
	if out, err := run("pnpm-missing install --prod"); err == nil || !strings.Contains(out, "is Node.js installed") {
		t.Fatalf("missing tool: %q %v", out, err)
	}
}

func TestCommandTool(t *testing.T) {
	for cmd, want := range map[string]string{
		"npm run build": "npm",
		"NEXT_TELEMETRY_DISABLED=1 npm run build": "npm",
		"A=1 B=two pnpm install":                  "pnpm",
		"./build.sh --prod":                       "./build.sh",
		"":                                        "",
		"NODE_ENV=production":                     "",
	} {
		if got := commandTool(cmd); got != want {
			t.Errorf("commandTool(%q) = %q, want %q", cmd, got, want)
		}
	}
}
