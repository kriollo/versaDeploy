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
	loaded   bool
	err      error
}

type msgDashboardData struct {
	current  string
	disk     string
	releases []string
	err      error
}

func loadDashboard(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		current := ""
		disk := ""
		var releases []string

		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		if target, err := client.ReadSymlink(currentSymlink); err == nil {
			current = filepath.Base(target)
		}

		dfCmd := fmt.Sprintf("df -h %q | tail -1 | awk '{print $3\"/\"$2\" (\"$5\" used)\"}'", remotePath)
		if out, err := client.ExecuteCommand(dfCmd); err == nil {
			disk = strings.TrimSpace(out)
		}

		releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
		releases, _ = client.ListReleases(releasesDir)

		return msgDashboardData{current: current, disk: disk, releases: releases}
	}
}

func (d *dashboardModel) applyData(msg msgDashboardData) {
	d.current = msg.current
	d.disk = msg.disk
	d.releases = msg.releases
	d.err = msg.err
	d.loaded = true
}

func (d dashboardModel) view(width int) string {
	if !d.loaded {
		return StyleMuted.Render("\n  Loading dashboard…")
	}
	if d.err != nil {
		return StyleError.Render("\n  Error: " + d.err.Error())
	}

	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	title := StyleTitle.Render("  Dashboard")

	current := StyleMuted.Render("—")
	if d.current != "" {
		current = StyleSuccess.Render(d.current)
	}
	disk := "—"
	if d.disk != "" {
		disk = d.disk
	}

	releaseCount := fmt.Sprintf("%d", len(d.releases))

	lines := []string{
		"",
		title,
		"",
		sep,
		"",
		fmt.Sprintf("  %-24s %s", "Current release:", current),
		fmt.Sprintf("  %-24s %s", "Disk usage:", disk),
		fmt.Sprintf("  %-24s %s", "Total releases:", releaseCount),
		"",
		sep,
		"",
		StyleMuted.Render("  Press 2 to manage releases, 5 to deploy"),
	}

	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
