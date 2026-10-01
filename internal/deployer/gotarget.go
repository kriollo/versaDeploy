package deployer

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/ssh"
)

// goarchesFor maps `uname -m` to the GOARCH values whose binaries run on that machine.
// 32-bit targets also run on their 64-bit counterpart (compat mode). uname reports the
// same "mips"/"mips64" for both endiannesses, so both are accepted there.
func goarchesFor(machine string) []string {
	switch {
	case machine == "x86_64" || machine == "amd64":
		return []string{"amd64", "386"}
	case len(machine) == 4 && machine[0] == 'i' && strings.HasSuffix(machine, "86"): // i386..i686
		return []string{"386"}
	case machine == "aarch64" || machine == "arm64":
		return []string{"arm64", "arm"}
	case strings.HasPrefix(machine, "arm"): // armv5tel, armv6l, armv7l, armv8l (32-bit)
		return []string{"arm"}
	case machine == "ppc64le", machine == "ppc64", machine == "s390x", machine == "riscv64":
		return []string{machine}
	case machine == "loongarch64":
		return []string{"loong64"}
	case machine == "mips64":
		return []string{"mips64", "mips64le"}
	case machine == "mips":
		return []string{"mips", "mipsle"}
	}
	return nil
}

// maxGOARM is the highest GOARM a 32-bit ARM machine can run (0 = no limit known).
func maxGOARM(machine string) int {
	if strings.HasPrefix(machine, "armv") && len(machine) > 4 {
		if v, err := strconv.Atoi(machine[4:5]); err == nil && v < 7 {
			return v
		}
	}
	return 0
}

// versionAtLeast compares the leading dotted numbers of v (e.g. "2.6.18-419.el5") to want.
func versionAtLeast(v string, want ...int) bool {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r < '0' || r > '9' })
	for i, w := range want {
		n := 0
		if i < len(parts) {
			n, _ = strconv.Atoi(parts[i])
		}
		if n != w {
			return n > w
		}
	}
	return true
}

// checkGoTarget verifies the configured Go target can run on the server: OS, arch,
// GOARM on old ARM boards, and the kernel minimum of the Go toolchain doing the build
// (Go 1.24+ needs Linux >= 3.2; https://go.dev/wiki/MinimumRequirements).
func (d *Deployer) checkGoTarget(c *ssh.Client) error {
	goCfg := d.env.Builds.Go
	if !goCfg.Enabled {
		return nil
	}
	out, err := c.ExecuteCommand("uname -srm")
	f := strings.Fields(out)
	if err != nil || len(f) < 3 {
		d.log.Warn("Could not detect server platform (continuing): %v %q", err, out)
		return nil
	}
	sysname, release, machine := strings.ToLower(f[0]), f[1], f[len(f)-1]
	d.log.Info("Server platform: %s %s %s", f[0], release, machine)

	fail := func(msg, hint string) error {
		return verserrors.New(verserrors.CodeConfigInvalid, msg, hint, nil)
	}
	if goCfg.TargetOS != sysname {
		return fail(fmt.Sprintf("go.target_os is %q but the server runs %s", goCfg.TargetOS, f[0]),
			fmt.Sprintf("Set go.target_os: %s in your deploy config.", sysname))
	}

	arches := goarchesFor(machine)
	if arches == nil {
		d.log.Warn("Unknown server architecture %q: cannot verify go.target_arch %q", machine, goCfg.TargetArch)
	} else if !contains(arches, goCfg.TargetArch) {
		return fail(fmt.Sprintf("go.target_arch is %q but the server is %s", goCfg.TargetArch, machine),
			fmt.Sprintf("Set go.target_arch: %s in your deploy config.", arches[0]))
	}

	if limit := maxGOARM(machine); limit > 0 && goCfg.TargetArch == "arm" {
		d.log.Warn("Server is %s: build with GOARM=%d or lower (export it before running versa), the default GOARM=7 crashes with 'illegal instruction'", machine, limit)
	}

	if goVer := d.localGoVersion(); goVer != "" && versionAtLeast(goVer, 1, 24) && !versionAtLeast(release, 3, 2) {
		return fail(fmt.Sprintf("%s binaries need Linux kernel 3.2 or newer, the server runs %s", goVer, release),
			"Build with Go 1.23 or older (e.g. run versa with GOTOOLCHAIN=go1.23.12; your go.mod 'go' line must allow it). Note: Go doesn't support kernels as old as 2.6.18 (RHEL/CentOS 5) in any current release.")
	}
	return nil
}

// localGoVersion returns the Go version (e.g. "go1.24.2") that builds the project,
// honoring go.mod's toolchain line and GOTOOLCHAIN. Empty if unknown.
func (d *Deployer) localGoVersion() string {
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Dir = filepath.Join(d.repoPath, d.env.Builds.Go.ProjectRoot)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
