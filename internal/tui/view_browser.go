package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type browserModel struct {
	stack   []string // directory stack; stack[0] = root (remotePath)
	entries []os.FileInfo
	cursor  int
	loaded  bool
	err     error
}

type msgDirListed struct {
	path    string
	entries []os.FileInfo
	err     error
}

func listDir(client *versassh.Client, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.ReadDir(path)
		return msgDirListed{path: path, entries: entries, err: err}
	}
}

func (b *browserModel) init(remotePath string) {
	b.stack = []string{remotePath}
	b.entries = nil
	b.cursor = 0
	b.loaded = false
}

func (b *browserModel) applyListed(msg msgDirListed) {
	b.entries = msg.entries
	b.err = msg.err
	b.loaded = true
	b.cursor = 0
}

func (b *browserModel) currentPath() string {
	if len(b.stack) == 0 {
		return "/"
	}
	return b.stack[len(b.stack)-1]
}

func (b *browserModel) moveUp() {
	if b.cursor > 0 {
		b.cursor--
	}
}

func (b *browserModel) moveDown() {
	if b.cursor < len(b.entries)-1 {
		b.cursor++
	}
}

// enterDir returns the path to enter (if the selected entry is a dir), or "".
func (b *browserModel) enterDir() string {
	if len(b.entries) == 0 || b.cursor >= len(b.entries) {
		return ""
	}
	e := b.entries[b.cursor]
	if e.IsDir() {
		return filepath.ToSlash(filepath.Join(b.currentPath(), e.Name()))
	}
	return ""
}

// pushDir pushes a new directory onto the stack.
func (b *browserModel) pushDir(path string) {
	b.stack = append(b.stack, path)
	b.loaded = false
}

// popDir pops the top directory off the stack.
func (b *browserModel) popDir() {
	if len(b.stack) > 1 {
		b.stack = b.stack[:len(b.stack)-1]
		b.loaded = false
	}
}

func (b browserModel) view(width int) string {
	title := StyleTitle.Render("  File Browser")
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	pathLine := StyleMuted.Render("  Path: ") + StyleActive.Render(b.currentPath())

	rows := []string{"", title, "", pathLine, "", sep}

	if !b.loaded {
		rows = append(rows, "", StyleMuted.Render("  Loading…"))
	} else if b.err != nil {
		rows = append(rows, "", StyleError.Render("  Error: "+b.err.Error()))
	} else if len(b.entries) == 0 {
		rows = append(rows, "", StyleMuted.Render("  (empty directory)"))
	} else {
		for i, e := range b.entries {
			icon := "  📄 "
			if e.IsDir() {
				icon = "  📁 "
			}
			size := ""
			if !e.IsDir() {
				size = StyleMuted.Render(fmt.Sprintf(" (%s)", humanSize(e.Size())))
			}
			name := e.Name()
			line := fmt.Sprintf("%s%s%s", icon, name, size)
			if i == b.cursor {
				line = StyleSelected.Render(fmt.Sprintf(" %s%-30s%s ", icon, name, size))
			}
			rows = append(rows, line)
		}
	}

	rows = append(rows, "", sep, "", StyleMuted.Render("  ↵:enter dir  ⌫:go up  ↑/↓:navigate"))

	return strings.Join(rows, "\n")
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
