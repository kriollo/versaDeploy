package ssh

import (
	"os/exec"
	"runtime"
	"testing"
)

// Runs the real probe under the local sh and busybox sh: core keys must have values.
func TestServerInfoScript(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs /proc")
	}
	shells := [][]string{{"sh", "-c"}}
	if _, err := exec.LookPath("busybox"); err == nil {
		shells = append(shells, []string{"busybox", "sh", "-c"})
	}
	for _, sh := range shells {
		out, err := exec.Command(sh[0], append(sh[1:], ServerInfoScript("/", true))...).Output()
		if err != nil {
			t.Fatalf("%s: script failed: %v", sh[0], err)
		}
		i := ParseServerInfo(string(out))
		for name, v := range map[string]string{
			"hostname": i.Hostname, "os": i.OS, "kernel": i.Kernel, "arch": i.Arch, "cores": i.Cores,
			"ram": i.RAM, "swap": i.Swap, "disk": i.Disk, "load": i.Load, "uptime": i.Uptime, "init": i.Init, "cpu": i.CPU,
		} {
			if v == "" {
				t.Errorf("%s: %s is empty; output:\n%s", sh[0], name, out)
			}
		}
		if !i.Has("tar") {
			t.Errorf("%s: tar not detected: %v", sh[0], i.Tools)
		}
	}
}

// Output captured from a CentOS 5 box (no MemAvailable, no os-release, no timeout).
func TestParseServerInfo(t *testing.T) {
	i := ParseServerInfo("hostname=legacy01\nuname=Linux 2.6.18-419.el5 i686\n" +
		"os=CentOS release 5.11 (Final)\ncores=2\nram=0.6G/2.0G used\nswap=none\n" +
		"init=sysvinit\nlibc=glibc 2.5\nrt_php=5.1.6\nrt_python=2.4.3\ninodes=\n" +
		"tools=tar gzip perl service \n")
	if i.Sysname != "Linux" || i.Kernel != "2.6.18-419.el5" || i.Arch != "i686" {
		t.Errorf("uname parsed as %q %q %q", i.Sysname, i.Kernel, i.Arch)
	}
	if i.Runtimes["php"] != "5.1.6" || i.Runtimes["python"] != "2.4.3" || len(i.Runtimes) != 2 {
		t.Errorf("runtimes %v", i.Runtimes)
	}
	if i.Has("timeout") || !i.Has("perl") || i.Inodes != "" || i.Libc != "glibc 2.5" {
		t.Errorf("tools %v inodes %q libc %q", i.Tools, i.Inodes, i.Libc)
	}
}
