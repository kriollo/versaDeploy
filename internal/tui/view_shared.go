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
	entries   []sharedEntry
	cursor    int
	viewStart int
	loaded    bool
	err       error
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
	s.cursor = 0
	s.viewStart = 0
}

func (s *sharedModel) moveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
}

func (s *sharedModel) moveDown() {
	if s.cursor < len(s.entries)-1 {
		s.cursor++
	}
}

func (s *sharedModel) selectedEntry() *sharedEntry {
	if len(s.entries) == 0 || s.cursor >= len(s.entries) {
		return nil
	}
	return &s.entries[s.cursor]
}

func (s sharedModel) view(width, height int) string {
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

		// Scroll window: overhead = title+sep+blank+header+blank = ~6, footer ~4
		vH := height - 10
		if vH < 3 {
			vH = 3
		}
		total := len(s.entries)

		if s.cursor >= s.viewStart+vH {
			s.viewStart = s.cursor - vH + 1
		} else if s.cursor < s.viewStart {
			s.viewStart = s.cursor
		}
		if s.viewStart < 0 {
			s.viewStart = 0
		}
		endIdx := s.viewStart + vH
		if endIdx > total {
			endIdx = total
		}

		for i := s.viewStart; i < endIdx; i++ {
			e := s.entries[i]
			icon := iconForEntry(e.name, e.isDir)
			line := fmt.Sprintf("  %s  %-36s %s", icon, e.name, e.size)
			if i == s.cursor {
				line = StyleSelected.Render(fmt.Sprintf("  %s  %-36s %s", icon, e.name, e.size))
			}
			rows = append(rows, line)
		}

		if total > vH {
			pct := 0
			if total > 1 {
				pct = int((float64(s.cursor) / float64(total-1)) * 100)
			}
			rows = append(rows, StyleMuted.Render(fmt.Sprintf(
				"  … %d/%d  (%d%%)", s.cursor+1, total, pct)))
		}
	}

	rows = append(rows, "", sep, "",
		StyleMuted.Render("  ↑/↓:navigate  ↵:open directory  F5:refresh"),
	)
	return strings.Join(rows, "\n")
}
