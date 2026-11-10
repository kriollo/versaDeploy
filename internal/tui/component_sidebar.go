package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const sidebarWidth = 18

// sideRow is one sidebar line under the title: a config file (env == "") or one of its
// environments. Rendering, ↑/↓ and clicks all use appModel.sidebarRows.
type sideRow struct {
	file string // absolute path; "" when the config didn't come from a file
	env  string
}

// sidebarRows lists every discovered deploy file with its environments, the loaded
// one first-class (its envNames are the source of truth). Without a file: just envs.
// ponytail: no scrolling, a directory with more rows than the screen gets clipped.
func (m appModel) sidebarRows() []sideRow {
	files := m.availableConfigs
	// the loaded file (--config, outside the directory); configPath alone is just the default
	if cur := m.config.configPath; m.cfg != nil && cur != "" && !containsPath(files, cur) {
		files = append([]string{cur}, files...)
	}
	var rows []sideRow
	if len(files) == 0 {
		for _, e := range m.envNames {
			rows = append(rows, sideRow{env: e})
		}
		return rows
	}
	for _, f := range files {
		rows = append(rows, sideRow{file: f})
		envs := m.fileEnvs[filepath.Clean(f)]
		if m.isCurrentFile(f) {
			envs = m.envNames
		}
		for _, e := range envs {
			rows = append(rows, sideRow{file: f, env: e})
		}
	}
	return rows
}

func (m appModel) isCurrentFile(f string) bool {
	return f == "" || filepath.Clean(f) == filepath.Clean(m.config.configPath)
}

// activeSideRow is the index in rows of the loaded file's active environment (-1: none).
func (m appModel) activeSideRow(rows []sideRow) int {
	for i, r := range rows {
		if r.env != "" && r.env == m.activeEnvName() && m.isCurrentFile(r.file) {
			return i
		}
	}
	return -1
}

func containsPath(files []string, p string) bool {
	for _, f := range files {
		if filepath.Clean(f) == filepath.Clean(p) {
			return true
		}
	}
	return false
}

// current is the loaded file: only its environments have a connection state.
func renderSidebar(height int, rows []sideRow, active int, current string, focused bool, states map[string]connState) string {
	out := []string{StyleTableHeader.Render("ENVIRONMENTS"), StyleMuted.Render(strings.Repeat("─", sidebarWidth-2))}
	for i, r := range rows {
		if r.env == "" {
			out = append(out, StyleTableHeader.Render(ansi.Truncate(filepath.Base(r.file), sidebarWidth-2, "…")))
			continue
		}
		indent := ""
		if r.file != "" {
			indent = " "
		}
		state := connIdle
		if r.file == "" || filepath.Clean(r.file) == filepath.Clean(current) {
			state = states[r.env]
		}
		line := fmt.Sprintf("%s%s %s", indent, connIndicator(state), ansi.Truncate(r.env, sidebarWidth-5-len(indent), ""))
		switch {
		case i == active && focused:
			line = StyleSelected.Render(" " + line + " ")
		case i == active:
			line = StyleActive.Render(line)
		default:
			line = StyleMuted.Render(line)
		}
		out = append(out, line)
	}
	for len(out) < height-2 {
		out = append(out, "")
	}
	return StyleSidebar.Width(sidebarWidth).Height(height).Render(strings.Join(out, "\n"))
}

func connIndicator(s connState) string {
	switch s {
	case connConnected:
		return StyleConnected.Render("\U000F012C") // mdi-check-circle
	case connConnecting:
		return StyleConnecting.Render("\U000F0765") // mdi-circle
	case connError:
		return StyleErrorState.Render("\U000F0159") // mdi-close-circle
	default:
		return StyleDisconnected.Render("\U000F0765") // mdi-circle (dim)
	}
}
