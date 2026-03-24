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
	releases  []string
	current   string
	cursor    int
	viewStart int
	loaded    bool
	err       error
	status    string
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
	r.viewStart = 0
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

func (r releasesModel) view(width, height int) string {
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

	// Reserve rows: 2 blank + title + blank + sep + blank + header = 6 overhead
	// plus sep + optional status + blank + footer = ~4 at bottom
	vH := height - 10
	if vH < 3 {
		vH = 3
	}

	total := len(r.releases)

	// Update scroll window
	if r.cursor >= r.viewStart+vH {
		r.viewStart = r.cursor - vH + 1
	} else if r.cursor < r.viewStart {
		r.viewStart = r.cursor
	}
	if r.viewStart < 0 {
		r.viewStart = 0
	}
	endIdx := r.viewStart + vH
	if endIdx > total {
		endIdx = total
	}

	for i := r.viewStart; i < endIdx; i++ {
		rel := r.releases[i]
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

	if total > vH {
		pct := 0
		if total > 1 {
			pct = int((float64(r.cursor) / float64(total-1)) * 100)
		}
		rows = append(rows, StyleMuted.Render(fmt.Sprintf(
			"  … %d/%d  (%d%%)", r.cursor+1, total, pct)))
	}

	rows = append(rows, "", sep)
	if r.status != "" {
		rows = append(rows, "", "  "+r.status)
	}
	rows = append(rows, "", StyleMuted.Render("  ↵:browse files  r:rollback  ↑/↓:navigate"))

	return strings.Join(rows, "\n")
}
