package ssh

import (
	"slices"
	"strings"
)

// ServerInfo describes the deploy server. Every field is best-effort: a probe that
// fails on some distro just leaves its field empty.
type ServerInfo struct {
	Hostname string
	OS       string // PRETTY_NAME, /etc/redhat-release or /etc/issue
	Sysname  string // uname -s, e.g. "Linux"
	Kernel   string // uname -r
	Arch     string // uname -m
	CPUModel string
	Cores    string
	CPU      string // usage %, only when requested (costs 1s)
	RAM      string
	Swap     string
	Disk     string // filesystem holding remote_path
	Inodes   string
	Load     string
	Uptime   string
	Init     string            // systemd, openrc, upstart or sysvinit
	Libc     string            // e.g. "glibc 2.17", "musl 1.2.4"
	Runtimes map[string]string // php, node, python, git → version
	Tools    []string          // which of serverTools are installed
}

// serverTools are the commands versa or common hooks depend on. "timeout" is probed
// separately: it is listed only if it takes `timeout -s SIG SECS CMD` (busybox before
// 1.30 wanted `-t SECS`).
const serverTools = "tar gzip perl python3 rsync systemctl service rc-service sudo"

// serverInfoScript prints one key=value line per probe in a single round-trip. It
// sticks to /proc and POSIX tools so it runs on old servers (RHEL/CentOS 5+, busybox):
// df -P avoids line wrapping with long LVM device names, /proc/meminfo replaces
// `free -h`, /proc/uptime replaces `uptime -p`, /proc/cpuinfo replaces `nproc`, and
// /etc/redhat-release covers distros without /etc/os-release. $p is the remote path,
// $cpu non-empty to sample CPU usage.
const serverInfoScript = `echo "hostname=$(uname -n)"
echo "uname=$(uname -srm)"
echo "os=$(if [ -f /etc/os-release ]; then grep '^PRETTY_NAME=' /etc/os-release | cut -d= -f2 | tr -d '"'; elif [ -f /etc/redhat-release ]; then cat /etc/redhat-release; else head -1 /etc/issue; fi)"
echo "cpu_model=$(awk -F': *' '/^(model name|Hardware|cpu model|Model)/{print $2; exit}' /proc/cpuinfo)"
echo "cores=$(grep -c '^processor' /proc/cpuinfo)"
awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} /^MemFree:/{f=$2} /^Buffers:/{b=$2} /^Cached:/{c=$2} /^SwapTotal:/{st=$2} /^SwapFree:/{sf=$2}
 END{if(!a)a=f+b+c; if(t)printf "ram=%.1fG/%.1fG used\n", (t-a)/1048576, t/1048576; if(st)printf "swap=%.1fG/%.1fG used\n", (st-sf)/1048576, st/1048576; else print "swap=none"}' /proc/meminfo
echo "disk=$(df -Ph "$p" 2>/dev/null | tail -1 | awk '{print $3"/"$2" ("$5" used)"}')"
echo "inodes=$(df -Pi "$p" 2>/dev/null | tail -1 | awk '$5 ~ /%/{print $5" used"}')"
echo "load=$(awk '{print $1", "$2", "$3}' /proc/loadavg)"
echo "uptime=$(awk '{s=int($1); printf "up %dd %dh %dm", s/86400, s%86400/3600, s%3600/60}' /proc/uptime)"
echo "init=$(if [ -d /run/systemd/system ]; then echo systemd; elif [ -x /sbin/openrc ] || [ -x /sbin/openrc-run ]; then echo openrc; elif [ -x /sbin/initctl ]; then echo upstart; else echo sysvinit; fi)"
l=$(ldd --version 2>&1 | head -2)
case "$l" in *musl*) echo "libc=musl $(echo "$l" | awk '/^Version/{print $2}')";; *GNU*|*GLIBC*) echo "libc=glibc $(echo "$l" | head -1 | awk '{print $NF}')";; esac
command -v php >/dev/null 2>&1 && echo "rt_php=$(php -v 2>/dev/null | head -1 | awk '{print $2}')"
command -v node >/dev/null 2>&1 && echo "rt_node=$(node -v 2>/dev/null)"
if command -v python3 >/dev/null 2>&1; then echo "rt_python=$(python3 -V 2>&1 | awk '{print $2}')"; elif command -v python >/dev/null 2>&1; then echo "rt_python=$(python -V 2>&1 | awk '{print $2}')"; fi
command -v git >/dev/null 2>&1 && echo "rt_git=$(git --version | awk '{print $3}')"
echo "tools=$(for t in ` + serverTools + `; do command -v $t >/dev/null 2>&1 && printf '%s ' $t; done; timeout -s KILL 5 true >/dev/null 2>&1 && printf timeout)"
[ -n "$cpu" ] && echo "cpu=$( (head -1 /proc/stat; sleep 1; head -1 /proc/stat) | awk '{t=0; for(k=2;k<=NF&&k<=9;k++)t+=$k; i=$5+$6; if(NR==1){t1=t;i1=i} else if(t>t1)printf "%.1f%%", 100*(1-(i-i1)/(t-t1))}')"
true`

// ServerInfoScript returns the probe script for remotePath; withCPU adds a 1s sample.
func ServerInfoScript(remotePath string, withCPU bool) string {
	cpu := ""
	if withCPU {
		cpu = "1"
	}
	return "p=" + ShellQuote(remotePath) + "; cpu=" + cpu + "\n" + serverInfoScript
}

// ServerInfo probes the server in one round-trip. It only errors when the command
// itself can't run; individual probes fail silently.
func (c *Client) ServerInfo(remotePath string, withCPU bool) (*ServerInfo, error) {
	out, err := c.ExecuteCommand(ServerInfoScript(remotePath, withCPU))
	if err != nil && out == "" {
		return nil, err
	}
	return ParseServerInfo(out), nil
}

// ParseServerInfo parses the key=value output of the probe script.
func ParseServerInfo(out string) *ServerInfo {
	kv := map[string]string{}
	info := &ServerInfo{Runtimes: map[string]string{}}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if v = strings.TrimSpace(v); !ok || v == "" {
			continue
		}
		if rt, ok := strings.CutPrefix(k, "rt_"); ok {
			info.Runtimes[rt] = v
		}
		kv[k] = v
	}
	if f := strings.Fields(kv["uname"]); len(f) >= 3 {
		// -r may contain spaces on odd kernels; the machine is always last
		info.Sysname, info.Kernel, info.Arch = f[0], strings.Join(f[1:len(f)-1], " "), f[len(f)-1]
	}
	info.Hostname, info.OS, info.CPUModel, info.Cores = kv["hostname"], kv["os"], kv["cpu_model"], kv["cores"]
	info.CPU, info.RAM, info.Swap, info.Disk, info.Inodes = kv["cpu"], kv["ram"], kv["swap"], kv["disk"], kv["inodes"]
	info.Load, info.Uptime, info.Init, info.Libc = kv["load"], kv["uptime"], kv["init"], kv["libc"]
	info.Tools = strings.Fields(kv["tools"])
	return info
}

// Has reports whether tool was found on the server's PATH.
func (i *ServerInfo) Has(tool string) bool {
	return slices.Contains(i.Tools, tool)
}
