package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type dashboardModel struct {
	current  string
	disk     string
	releases []string
	// Server stats
	ram    string
	cpu    string
	load   string
	uptime string
	os     string
	loaded bool
	err    error
}

type msgDashboardData struct {
	current  string
	disk     string
	releases []string
	ram      string
	cpu      string
	load     string
	uptime   string
	os       string
	err      error
}

// serverStatsScript prints one key=value line per stat in a single SSH round-trip.
// It sticks to /proc and POSIX tools so it works on old servers (RHEL/CentOS 5+):
// df -P avoids line wrapping with long LVM device names, /proc/meminfo replaces
// `free -h`, /proc/uptime replaces `uptime -p`, and /etc/redhat-release covers
// distros without /etc/os-release. %s is the quoted remote path.
const serverStatsScript = `echo "disk=$(df -Ph %s | tail -1 | awk '{print $3"/"$2" ("$5" used)"}')"
echo "ram=$(awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} /^MemFree:/{f=$2} /^Buffers:/{b=$2} /^Cached:/{c=$2} END{if(!a)a=f+b+c; if(t)printf "%%.1fG/%%.1fG used", (t-a)/1048576, t/1048576}' /proc/meminfo)"
echo "load=$(awk '{print $1", "$2", "$3}' /proc/loadavg)"
echo "uptime=$(awk '{s=int($1); printf "up %%dd %%dh %%dm", s/86400, s%%86400/3600, s%%3600/60}' /proc/uptime)"
echo "os=$(if [ -f /etc/os-release ]; then grep '^PRETTY_NAME=' /etc/os-release | cut -d= -f2 | tr -d '"'; elif [ -f /etc/redhat-release ]; then cat /etc/redhat-release; else head -1 /etc/issue; fi)"
echo "cpu=$( (head -1 /proc/stat; sleep 1; head -1 /proc/stat) | awk '{t=0; for(k=2;k<=NF&&k<=9;k++)t+=$k; i=$5+$6; if(NR==1){t1=t;i1=i} else if(t>t1)printf "%%.1f%%%%", 100*(1-(i-i1)/(t-t1))}')"`

// parseStats parses key=value lines; values may contain '='.
func parseStats(out string) map[string]string {
	stats := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			stats[k] = strings.TrimSpace(v)
		}
	}
	return stats
}

func loadDashboard(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		current := ""
		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		if target, err := client.ReadSymlink(currentSymlink); err == nil {
			current = filepath.Base(target)
		}

		releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
		releases, _ := client.ListReleases(releasesDir)

		// Each stat is best-effort: a failing line just leaves its field empty
		out, _ := client.ExecuteCommand(fmt.Sprintf(serverStatsScript, versassh.ShellQuote(remotePath)))
		stats := parseStats(out)
		if len(stats["uptime"]) > 40 {
			stats["uptime"] = stats["uptime"][:40] + "…"
		}

		return msgDashboardData{
			current:  current,
			disk:     stats["disk"],
			releases: releases,
			ram:      stats["ram"],
			cpu:      stats["cpu"],
			load:     stats["load"],
			uptime:   stats["uptime"],
			os:       stats["os"],
		}
	}
}

func (d *dashboardModel) applyData(msg msgDashboardData) {
	d.current = msg.current
	d.disk = msg.disk
	d.releases = msg.releases
	d.ram = msg.ram
	d.cpu = msg.cpu
	d.load = msg.load
	d.uptime = msg.uptime
	d.os = msg.os
	d.err = msg.err
	d.loaded = true
}

func stat(label, value string) string {
	v := value
	if v == "" {
		v = StyleMuted.Render("—")
	}
	return fmt.Sprintf("  %-24s %s", label, v)
}

func (d dashboardModel) view(width, _ int) string {
	if !d.loaded {
		return StyleMuted.Render("\n  Loading dashboard…")
	}
	if d.err != nil {
		return StyleError.Render("\n  Error: " + errDisplay(d.err))
	}

	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	title := StyleTitle.Render("  Dashboard")

	currentVal := StyleMuted.Render("—")
	if d.current != "" {
		currentVal = StyleSuccess.Render(d.current)
	}

	releaseCount := fmt.Sprintf("%d", len(d.releases))

	lines := []string{
		"",
		title,
		"",
		sep,
		"",
		StyleSection.Render("  Deployment"),
		"",
		stat("Current release:", currentVal),
		stat("Total releases:", releaseCount),
		"",
		sep,
		"",
		StyleSection.Render("  Server Resources"),
		"",
	}

	if d.os != "" {
		lines = append(lines, stat("OS:", d.os))
	}
	lines = append(lines,
		stat("CPU usage:", d.cpu),
		stat("RAM usage:", d.ram),
		stat("Load (1/5/15m):", d.load),
		stat("Uptime:", d.uptime),
		stat("Disk (deploy):", d.disk),
	)

	lines = append(lines,
		"",
		sep,
		"",
		StyleMuted.Render("  2=releases  5=deploy  F5=refresh"),
	)

	return strings.Join(lines, "\n")
}
