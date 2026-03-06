package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/state"
)

// Ensure state is used (SortReleases is the canonical sort)
var _ = state.SortReleases

type releasesModel struct {
	releases []string
	current  string
	cursor   int
	loaded   bool
	err      error
	status   string
}

type msgReleasesLoaded struct {
	releases []string
	current  string
	err      error
}

func loadReleases(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
		releases, err := client.ListReleases(releasesDir)
		if err != nil {
			return msgReleasesLoaded{err: err}
		}

		state.SortReleases(releases)

		current := ""
		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		if target, e := client.ReadSymlink(currentSymlink); e == nil {
			current = filepath.Base(target)
		}

		return msgReleasesLoaded{releases: releases, current: current}
	}
}

func (r *releasesModel) applyLoaded(msg msgReleasesLoaded) {
	r.releases = msg.releases
	r.current = msg.current
	r.err = msg.err
	r.loaded = true
	r.cursor = 0
}

func (r *releasesModel) moveUp() {
	if r.cursor > 0 {
		r.cursor--
	}
}

func (r *releasesModel) moveDown() {
	if r.cursor < len(r.releases)-1 {
		r.cursor++
	}
}

func (r releasesModel) selectedRelease() string {
	if len(r.releases) == 0 || r.cursor >= len(r.releases) {
		return ""
	}
	return r.releases[r.cursor]
}

func (r releasesModel) view(width int) string {
	if !r.loaded {
		return StyleMuted.Render("\n  Loading releases…")
	}
	if r.err != nil {
		return StyleError.Render("\n  Error: " + r.err.Error())
	}
	if len(r.releases) == 0 {
		return StyleMuted.Render("\n  No releases found.")
	}

	title := StyleTitle.Render("  Releases")
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	// Column header
	header := StyleTableHeader.Render(fmt.Sprintf("  %-3s %-26s %s", "#", "Release", "Status"))

	rows := []string{"", title, "", sep, "", header}

	for i, rel := range r.releases {
		num := fmt.Sprintf("%d", i+1)
		marker := "  "
		status := ""
		if rel == r.current {
			marker = StyleSuccess.Render("→ ")
			status = StyleSuccess.Render("current")
		}

		line := fmt.Sprintf("  %s%-3s %-26s %s", marker, num, rel, status)
		if i == r.cursor {
			line = StyleSelected.Render(fmt.Sprintf(" %-3s %-26s %-10s", num, rel, status))
		}
		rows = append(rows, line)
	}

	rows = append(rows, "", sep)
	if r.status != "" {
		rows = append(rows, "", "  "+r.status)
	}
	rows = append(rows, "", StyleMuted.Render("  r:rollback to selected  ↑/↓:navigate"))

	return strings.Join(rows, "\n")
}
