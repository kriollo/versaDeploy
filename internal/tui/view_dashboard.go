package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type dashboardModel struct {
	msgDashboardData
	loaded bool
	offset int // first visible line when the dashboard is taller than the pane
}

type msgDashboardData struct {
	envName  string
	current  string
	releases []string
	server   *versassh.ServerInfo
	err      error
}

func loadDashboard(client *versassh.Client, envName, remotePath string) tea.Cmd {
	return func() tea.Msg {
		msg := msgDashboardData{envName: envName}
		if target, err := client.ReadSymlink(remotePath + "/current"); err == nil {
			msg.current = filepath.Base(target)
		}
		msg.releases, _ = client.ListReleases(remotePath + "/releases") // missing before the first deploy
		msg.server, msg.err = client.ServerInfo(remotePath, true)
		return msg
	}
}

func (d *dashboardModel) applyData(msg msgDashboardData) {
	d.msgDashboardData = msg
	d.loaded = true
}

// scroll moves the visible window by delta lines, clamped to the content.
func (d *dashboardModel) scroll(delta, height int) {
	d.offset = clampOffset(d.offset+delta, len(d.lines(0)), height)
}

func clampOffset(offset, total, height int) int {
	return max(0, min(offset, total-height))
}

func stat(label, value string) string {
	v := value
	if v == "" {
		v = StyleMuted.Render("—")
	}
	return fmt.Sprintf("  %-24s %s", label, v)
}

func (d dashboardModel) view(width, height int) string {
	lines := d.lines(width)
	if height <= 0 || len(lines) <= height {
		return strings.Join(lines, "\n")
	}

	// Taller than the pane: show a window and mark the hidden parts, so the
	// overflow doesn't push the header off screen.
	off := clampOffset(d.offset, len(lines), height)
	visible := append([]string(nil), lines[off:off+height]...)
	if off > 0 {
		visible[0] = StyleMuted.Render("  ↑ more")
	}
	if off+height < len(lines) {
		visible[height-1] = StyleMuted.Render("  ↓ more  (↑/↓ PgUp/PgDn to scroll)")
	}
	return strings.Join(visible, "\n")
}

func (d dashboardModel) lines(width int) []string {
	if !d.loaded {
		return []string{"", StyleMuted.Render("  Loading dashboard…")}
	}
	if d.err != nil {
		return []string{"", StyleError.Render("  Error: " + errDisplay(d.err))}
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

	srv := d.server
	cpu := srv.CPUModel
	if srv.Cores != "" {
		cpu += " (" + srv.Cores + " cores)"
	}
	lines = append(lines,
		stat("Host:", srv.Hostname),
		stat("OS:", srv.OS),
		stat("Kernel:", strings.TrimSpace(srv.Kernel+" "+srv.Arch)),
		stat("Init / libc:", strings.Trim(srv.Init+" / "+srv.Libc, " /")),
		stat("CPU:", cpu),
		stat("CPU usage:", srv.CPU),
		stat("RAM usage:", srv.RAM),
		stat("Swap:", srv.Swap),
		stat("Load (1/5/15m):", srv.Load),
		stat("Uptime:", srv.Uptime),
		stat("Disk (deploy):", srv.Disk),
		stat("Inodes (deploy):", srv.Inodes),
		"",
		sep,
		"",
		StyleSection.Render("  Runtimes & tools"),
		"",
	)
	for _, rt := range []string{"php", "node", "python", "git"} {
		lines = append(lines, stat(rt+":", srv.Runtimes[rt]))
	}
	lines = append(lines, stat("tools:", strings.Join(srv.Tools, " ")))

	lines = append(lines,
		"",
		sep,
		"",
		StyleMuted.Render("  2=releases  5=deploy  F5=refresh"),
	)

	return lines
}
