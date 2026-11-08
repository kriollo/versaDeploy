package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type sharedEntry struct {
	name  string
	size  string
	isDir bool
	path  string      // absolute remote path
	info  os.FileInfo // nil when missing
	note  string      // "preserved", "missing"
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

// loadShared lists the shared/ directory on the remote server, then the files the
// config keeps between releases outside it: preserved_paths (e.g. config.json, in
// current/app) and shared_paths not created yet (the next deploy creates them).
func loadShared(client *versassh.Client, remotePath string, sharedPaths, preserved []string) tea.Cmd {
	return func() tea.Msg {
		sharedBase := filepath.ToSlash(filepath.Join(remotePath, "shared"))

		// shared/ may not exist yet — not an error per se
		fileInfos, _ := client.ReadDir(sharedBase)

		paths := make([]string, len(fileInfos))
		for i, fi := range fileInfos {
			paths[i] = filepath.ToSlash(filepath.Join(sharedBase, fi.Name()))
		}
		sizes, _ := client.DirSizesKB(paths) // one round-trip for all entries

		entries := make([]sharedEntry, 0, len(fileInfos))
		seen := map[string]bool{}
		for i, fi := range fileInfos {
			size := "—"
			if !fi.IsDir() {
				size = humanSize(fi.Size()) // du rounds a file up to its blocks
			} else if kb, ok := sizes[paths[i]]; ok {
				size = humanSize(kb * 1024)
			}
			seen[fi.Name()] = true
			entries = append(entries, sharedEntry{name: fi.Name(), size: size, isDir: fi.IsDir(), path: paths[i], info: fi})
		}
		for _, p := range sharedPaths {
			p = filepath.ToSlash(filepath.Clean(p))
			if top := strings.SplitN(p, "/", 2)[0]; !seen[top] {
				seen[top] = true
				entries = append(entries, sharedEntry{name: p, size: "—", note: "missing"})
			}
		}
		for _, p := range preserved {
			p = filepath.ToSlash(filepath.Clean(p))
			full := filepath.ToSlash(filepath.Join(remotePath, "current", "app", p))
			e := sharedEntry{name: p, size: "—", path: full, note: "preserved"}
			if fi, err := client.Stat(full); err == nil {
				e.info, e.isDir, e.size = fi, fi.IsDir(), humanSize(fi.Size())
			} else {
				e.note, e.path = "missing", ""
			}
			entries = append(entries, e)
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
		rows = append(rows, StyleError.Render("  Error: "+errDisplay(s.err)))
	} else if len(s.entries) == 0 {
		rows = append(rows, StyleMuted.Render("  The shared/ directory is empty or does not exist yet."))
		rows = append(rows, "", StyleHint.Render("  Add 'shared_paths' to your deploy.yml to persist"))
		rows = append(rows, StyleHint.Render("  directories (e.g. storage, uploads) between releases."))
	} else {
		header := StyleTableHeader.Render(fmt.Sprintf("  %-4s %-36s %-10s %s", "Type", "Name", "Size", ""))
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
			line := fmt.Sprintf("  %s  %-36s %-10s %s", icon, e.name, e.size, e.note)
			if i == s.cursor {
				line = StyleSelected.Render(line)
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
		StyleMuted.Render("  ↑/↓:navigate  ↵:open (files: e edits .env, config.json…)  F5:refresh"),
	)
	return strings.Join(rows, "\n")
}
