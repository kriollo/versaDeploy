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
	ram     string
	cpu     string
	load    string
	uptime  string
	os      string
	loaded  bool
	err     error
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

func loadDashboard(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		current := ""
		disk := ""
		ram := ""
		cpu := ""
		load := ""
		uptime := ""
		osInfo := ""
		var releases []string

		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		if target, err := client.ReadSymlink(currentSymlink); err == nil {
			current = filepath.Base(target)
		}

		// Disk usage for remote path
		dfCmd := fmt.Sprintf("df -h %q | tail -1 | awk '{print $3\"/\"$2\" (\"$5\" used)\"}'", remotePath)
		if out, err := client.ExecuteCommand(dfCmd); err == nil {
			disk = strings.TrimSpace(out)
		}

		releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
		releases, _ = client.ListReleases(releasesDir)

		// RAM: free -h → total and used
		if out, err := client.ExecuteCommand("free -h 2>/dev/null | awk '/^Mem:/{print $3\"/\"$2\" used\"}'"); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				ram = v
			}
		}

		// CPU: single-shot mpstat or fallback to /proc/stat
		cpuCmd := `mpstat 1 1 2>/dev/null | awk '/Average:/{printf "%.1f%%", 100-$NF}' || awk '/cpu /{u=$2+$4; t=$2+$3+$4+$5; printf "%.1f%%", (u/t)*100; exit}' /proc/stat`
		if out, err := client.ExecuteCommand(cpuCmd); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				cpu = v
			}
		}

		// Load average
		if out, err := client.ExecuteCommand("cat /proc/loadavg 2>/dev/null | awk '{print $1\", \"$2\", \"$3}'"); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				load = v
			}
		}

		// Uptime
		if out, err := client.ExecuteCommand("uptime -p 2>/dev/null || uptime"); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				if len(v) > 40 {
					v = v[:40] + "…"
				}
				uptime = v
			}
		}

		// OS info
		if out, err := client.ExecuteCommand("cat /etc/os-release 2>/dev/null | grep '^PRETTY_NAME' | cut -d= -f2 | tr -d '\"'"); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				osInfo = v
			}
		}

		return msgDashboardData{
			current:  current,
			disk:     disk,
			releases: releases,
			ram:      ram,
			cpu:      cpu,
			load:     load,
			uptime:   uptime,
			os:       osInfo,
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
		return StyleError.Render("\n  Error: " + d.err.Error())
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
