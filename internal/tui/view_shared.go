package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type sharedPathInfo struct {
	path string
	size string
}

type sharedModel struct {
	paths  []sharedPathInfo
	loaded bool
	err    error
}

type msgSharedData struct {
	paths []sharedPathInfo
	err   error
}

func loadShared(client *versassh.Client, remotePath string, sharedPaths []string) tea.Cmd {
	return func() tea.Msg {
		sharedBase := filepath.ToSlash(filepath.Join(remotePath, "shared"))
		var infos []sharedPathInfo

		for _, p := range sharedPaths {
			fullPath := filepath.ToSlash(filepath.Join(sharedBase, p))
			size := "—"
			if out, err := client.ExecuteCommand(fmt.Sprintf("du -sh %q 2>/dev/null | awk '{print $1}'", fullPath)); err == nil {
				size = strings.TrimSpace(out)
			}
			infos = append(infos, sharedPathInfo{path: p, size: size})
		}

		return msgSharedData{paths: infos}
	}
}

func (s *sharedModel) applyData(msg msgSharedData) {
	s.paths = msg.paths
	s.err = msg.err
	s.loaded = true
}

func (s sharedModel) view(width int) string {
	title := StyleTitle.Render("  Shared Paths")
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	rows := []string{"", title, "", sep, ""}

	if !s.loaded {
		rows = append(rows, StyleMuted.Render("  Loading…"))
	} else if s.err != nil {
		rows = append(rows, StyleError.Render("  Error: "+s.err.Error()))
	} else if len(s.paths) == 0 {
		rows = append(rows, StyleMuted.Render("  No shared paths configured."))
		rows = append(rows, "", StyleMuted.Render("  Add 'shared_paths' to your deploy.yml to persist directories between releases."))
	} else {
		header := StyleTableHeader.Render(fmt.Sprintf("  %-36s %s", "Path", "Size"))
		rows = append(rows, header, "")
		for _, p := range s.paths {
			rows = append(rows, fmt.Sprintf("  %-36s %s", p.path, p.size))
		}
	}

	rows = append(rows, "", sep)
	return strings.Join(rows, "\n")
}
