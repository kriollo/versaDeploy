package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type sharedEntry struct {
	name  string
	size  string
	isDir bool
}

type sharedModel struct {
	entries []sharedEntry
	loaded  bool
	err     error
}

type msgSharedData struct {
	entries []sharedEntry
	err     error
}

// loadShared lists the shared/ directory on the remote server.
func loadShared(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		sharedBase := filepath.ToSlash(filepath.Join(remotePath, "shared"))

		fileInfos, err := client.ReadDir(sharedBase)
		if err != nil {
			// shared/ may not exist yet — not an error per se
			return msgSharedData{entries: nil, err: nil}
		}

		entries := make([]sharedEntry, 0, len(fileInfos))
		for _, fi := range fileInfos {
			size := "—"
			fullPath := filepath.ToSlash(filepath.Join(sharedBase, fi.Name()))
			if out, e := client.ExecuteCommand(
				fmt.Sprintf("du -sh %q 2>/dev/null | awk '{print $1}'", fullPath),
			); e == nil {
				size = strings.TrimSpace(out)
			}
			entries = append(entries, sharedEntry{
				name:  fi.Name(),
				size:  size,
				isDir: fi.IsDir(),
			})
		}

		return msgSharedData{entries: entries}
	}
}

func (s *sharedModel) applyData(msg msgSharedData) {
	s.entries = msg.entries
	s.err = msg.err
	s.loaded = true
}

func (s sharedModel) view(width int) string {
	title := StyleTitle.Render("  Shared Directory")
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	rows := []string{"", title, "", sep, ""}

	if !s.loaded {
		rows = append(rows, StyleMuted.Render("  Loading…"))
	} else if s.err != nil {
		rows = append(rows, StyleError.Render("  Error: "+s.err.Error()))
	} else if len(s.entries) == 0 {
		rows = append(rows, StyleMuted.Render("  The shared/ directory is empty or does not exist yet."))
		rows = append(rows, "", StyleHint.Render("  Add 'shared_paths' to your deploy.yml to persist"))
		rows = append(rows, StyleHint.Render("  directories (e.g. storage, uploads) between releases."))
	} else {
		header := StyleTableHeader.Render(fmt.Sprintf("  %-4s %-36s %s", "Type", "Name", "Size"))
		rows = append(rows, header, "")
		for _, e := range s.entries {
			icon := "📄"
			if e.isDir {
				icon = "📁"
			}
			rows = append(rows, fmt.Sprintf("  %s   %-36s %s", icon, e.name, e.size))
		}
	}

	rows = append(rows, "", sep)
	return strings.Join(rows, "\n")
}
