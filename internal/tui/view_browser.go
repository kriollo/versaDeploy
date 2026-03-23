package tui

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type msgDirListed struct {
	path    string
	entries []os.FileInfo
	err     error
}

type msgFileContent struct {
	path    string
	content []byte
	info    os.FileInfo
	err     error
}

// ── Browser model ─────────────────────────────────────────────────────────────

type browserModel struct {
	// Directory navigation
	stack     []string
	entries   []os.FileInfo
	cursor    int
	viewStart int // Add pagination tracking
	loaded    bool
	err       error

	// File viewer
	viewing         bool
	viewPath        string
	viewInfo        os.FileInfo
	viewContent     []byte
	viewLoading     bool
	viewErr         error
	viewViewport    viewport.Model
	viewInitialized bool
}

// ── SSH commands ──────────────────────────────────────────────────────────────

func listDir(client *versassh.Client, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.ReadDir(path)
		return msgDirListed{path: path, entries: entries, err: err}
	}
}

func loadFileContent(client *versassh.Client, path string, info os.FileInfo) tea.Cmd {
	return func() tea.Msg {
		const maxRead = 512 * 1024 // 512 KB max
		data, err := client.ReadRemoteBytes(path, maxRead)
		return msgFileContent{path: path, content: data, info: info, err: err}
	}
}

// ── Model methods ─────────────────────────────────────────────────────────────

func (b *browserModel) init(remotePath string) {
	b.stack = []string{remotePath}
	b.entries = nil
	b.cursor = 0
	b.viewStart = 0
	b.loaded = false
	b.viewing = false
}

func (b *browserModel) applyListed(msg msgDirListed) {
	b.entries = msg.entries
	b.err = msg.err
	b.loaded = true
	b.cursor = 0
	b.viewStart = 0
	b.viewing = false
}

func (b *browserModel) applyFileContent(msg msgFileContent, viewW, viewH int) {
	b.viewContent = msg.content
	b.viewInfo = msg.info
	b.viewErr = msg.err
	b.viewLoading = false

	if !b.viewInitialized {
		b.viewViewport = viewport.New(viewW-4, viewH-10)
		b.viewInitialized = true
	} else {
		b.viewViewport.Width = viewW - 4
		b.viewViewport.Height = viewH - 10
	}

	if msg.err == nil {
		b.viewViewport.SetContent(b.renderFileContent())
	}
	b.viewViewport.GotoTop()
}

func (b *browserModel) currentPath() string {
	if len(b.stack) == 0 {
		return "/"
	}
	return b.stack[len(b.stack)-1]
}

func (b *browserModel) hasParent() bool { return len(b.stack) > 1 }

// totalVisible returns the number of visible rows (including virtual ".." entry).
func (b *browserModel) totalVisible() int {
	n := len(b.entries)
	if b.hasParent() {
		n++ // ".." virtual entry
	}
	return n
}

// entryAt maps a visual cursor position to a real os.FileInfo.
// Returns nil for the ".." virtual entry.
func (b *browserModel) entryAt(pos int) os.FileInfo {
	if b.hasParent() {
		if pos == 0 {
			return nil // ".." virtual entry
		}
		return b.entries[pos-1]
	}
	return b.entries[pos]
}

func (b *browserModel) moveUp() {
	if b.viewing {
		b.viewViewport.LineUp(3)
		return
	}
	if b.cursor > 0 {
		b.cursor--
	}
}

func (b *browserModel) moveDown() {
	if b.viewing {
		b.viewViewport.LineDown(3)
		return
	}
	if b.cursor < b.totalVisible()-1 {
		b.cursor++
	}
}

// enterSelected returns either a dir path to navigate into, or a file to view.
// Returns (dirPath, filePath, fileInfo).
func (b *browserModel) enterSelected() (dirPath string, filePath string, info os.FileInfo) {
	if b.totalVisible() == 0 {
		return
	}
	entry := b.entryAt(b.cursor)
	if entry == nil {
		// ".." virtual entry — go up
		return "", "", nil
	}
	fullPath := filepath.ToSlash(filepath.Join(b.currentPath(), entry.Name()))
	if entry.IsDir() {
		return fullPath, "", nil
	}
	return "", fullPath, entry
}

func (b *browserModel) pushDir(path string) {
	b.stack = append(b.stack, path)
	b.loaded = false
	b.viewing = false
}

func (b *browserModel) popDir() {
	if b.viewing {
		b.viewing = false
		return
	}
	if len(b.stack) > 1 {
		b.stack = b.stack[:len(b.stack)-1]
		b.loaded = false
	}
}

// ── File type helpers ─────────────────────────────────────────────────────────

type fileKind int

const (
	kindText fileKind = iota
	kindImage
	kindBinary
)

var textExts = map[string]bool{
	".txt": true, ".log": true, ".md": true, ".go": true, ".php": true,
	".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".css": true,
	".html": true, ".htm": true, ".yml": true, ".yaml": true, ".json": true,
	".py": true, ".rb": true, ".sh": true, ".bash": true, ".zsh": true,
	".fish": true, ".env": true, ".ini": true, ".conf": true, ".toml": true,
	".xml": true, ".sql": true, ".lock": true, ".dockerfile": true,
	".gitignore": true, ".gitattributes": true, ".editorconfig": true,
	".vue": true, ".svelte": true, ".rs": true, ".java": true, ".c": true,
	".h": true, ".cpp": true, ".cs": true, ".kt": true, ".swift": true,
}

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".webp": true, ".svg": true, ".ico": true, ".tiff": true,
}

func detectKind(name string) fileKind {
	ext := strings.ToLower(filepath.Ext(name))
	if textExts[ext] {
		return kindText
	}
	if imageExts[ext] {
		return kindImage
	}
	return kindBinary
}

func (b *browserModel) renderFileContent() string {
	if b.viewErr != nil {
		return StyleError.Render("Error reading file: " + b.viewErr.Error())
	}
	name := filepath.Base(b.viewPath)
	switch detectKind(name) {
	case kindText:
		if !utf8.Valid(b.viewContent) {
			return StyleWarning.Render("File contains non-UTF8 data — showing as binary.\n\n") +
				hexDump(b.viewContent, 512)
		}
		return string(b.viewContent)
	case kindImage:
		return renderImageInfo(name, b.viewInfo, b.viewContent)
	default:
		return StyleMuted.Render("Binary file — hex preview:\n\n") +
			hexDump(b.viewContent, 512)
	}
}

func renderImageInfo(name string, info os.FileInfo, data []byte) string {
	lines := []string{
		StyleTitle.Render("Image file"),
		"",
		fmt.Sprintf("  Name:  %s", name),
	}
	if info != nil {
		lines = append(lines, fmt.Sprintf("  Size:  %s", humanSize(info.Size())))
		lines = append(lines, fmt.Sprintf("  Mtime: %s", info.ModTime().Format("2006-01-02 15:04:05")))
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".svg" && len(data) > 0 && utf8.Valid(data) {
		lines = append(lines, "", StyleMuted.Render("SVG content preview:"), "")
		preview := string(data)
		if len(preview) > 1024 {
			preview = preview[:1024] + "\n…(truncated)"
		}
		lines = append(lines, preview)
	} else {
		lines = append(lines,
			"",
			StyleMuted.Render("  Raster images cannot be rendered in this terminal."),
			StyleMuted.Render("  Use scp/rsync to copy the file locally for viewing."),
		)
		// Show magic bytes as hint
		if len(data) >= 4 {
			lines = append(lines, "",
				StyleMuted.Render(fmt.Sprintf("  Magic bytes: % X", data[:min(8, len(data))])),
			)
		}
	}
	return strings.Join(lines, "\n")
}

func hexDump(data []byte, maxBytes int) string {
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	var sb strings.Builder
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		h := hex.EncodeToString(chunk)
		// Space-separate pairs
		var hexParts []string
		for j := 0; j < len(h); j += 2 {
			hexParts = append(hexParts, h[j:j+2])
		}
		// ASCII column
		ascii := make([]byte, len(chunk))
		for j, c := range chunk {
			if c >= 32 && c < 127 {
				ascii[j] = c
			} else {
				ascii[j] = '.'
			}
		}
		sb.WriteString(fmt.Sprintf("  %08x  %-48s  %s\n",
			i, strings.Join(hexParts, " "), string(ascii)))
	}
	if len(data) == maxBytes {
		sb.WriteString(StyleMuted.Render(fmt.Sprintf("\n  … (showing first %d bytes)", maxBytes)))
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── View ──────────────────────────────────────────────────────────────────────

func (b browserModel) view(width int) string {
	if b.viewing {
		return b.viewFileView(width)
	}
	return b.dirView(width)
}

func (b browserModel) dirView(width int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	title := StyleTitle.Render("  File Browser")
	pathLine := StyleMuted.Render("  Path: ") + StyleActive.Render(b.currentPath())

	rows := []string{"", title, "", pathLine, "", sep}

	if !b.loaded {
		rows = append(rows, "", StyleMuted.Render("  Loading…"))
	} else if b.err != nil {
		rows = append(rows, "", StyleError.Render("  Error: "+b.err.Error()))
	} else {
		totalVisible := b.totalVisible()
		if totalVisible == 0 {
			rows = append(rows, "", StyleMuted.Render("  (empty directory)"))
		} else {
			// Calculate viewport height (approximate based on terminal space)
			vH := heightFromWidth(width) + 5
			if vH > totalVisible {
				vH = totalVisible
			}

			// Scroll tracking
			if b.cursor >= b.viewStart+vH {
				b.viewStart = b.cursor - vH + 1
			} else if b.cursor < b.viewStart {
				b.viewStart = b.cursor
			}

			if b.viewStart < 0 {
				b.viewStart = 0
			}

			endIdx := b.viewStart + vH
			if endIdx > totalVisible {
				endIdx = totalVisible
			}

			for i := b.viewStart; i < endIdx; i++ {
				entry := b.entryAt(i)

				var line string
				if entry == nil {
					// Virtual ".." entry
					icon := "  📂 "
					name := ".."
					if i == b.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, name))
					} else {
						line = StyleMuted.Render(fmt.Sprintf("%s%s", icon, name))
					}
				} else {
					icon := "  📄 "
					if entry.IsDir() {
						icon = "  📁 "
					}
					name := entry.Name()
					if utf8.RuneCountInString(name) > 28 {
						runes := []rune(name)
						name = string(runes[:25]) + "..."
					}

					size := ""
					if !entry.IsDir() {
						size = StyleMuted.Render(fmt.Sprintf("%9s", humanSize(entry.Size())))
					} else {
						size = strings.Repeat(" ", 9)
					}
					modTime := StyleMuted.Render(entry.ModTime().Format(" 2006-01-02"))

					if i == b.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, name)) +
							size + modTime
					} else {
						line = fmt.Sprintf("%s%-30s%s%s", icon, name, size, modTime)
					}
				}
				rows = append(rows, line)
			}

			// Show scroll hint if there are more files
			if totalVisible > vH {
				pct := int((float64(b.cursor) / float64(totalVisible-1)) * 100)
				rows = append(rows, StyleMuted.Render(fmt.Sprintf("  ... %d more files (%d%%)", totalVisible-endIdx, pct)))
			}
		}
	}

	rows = append(rows, "", sep, "",
		StyleMuted.Render("  ↵:open  ⌫:go up  ↑/↓ or j/k:navigate"),
	)
	return strings.Join(rows, "\n")
}

func (b browserModel) viewFileView(width int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	name := filepath.Base(b.viewPath)
	title := StyleTitle.Render("  " + name)

	rows := []string{"", title, ""}

	if b.viewInfo != nil {
		rows = append(rows,
			fmt.Sprintf("  Size: %-14s  Modified: %s",
				humanSize(b.viewInfo.Size()),
				b.viewInfo.ModTime().Format("2006-01-02 15:04:05"),
			),
		)
	}
	rows = append(rows, "", sep, "")

	if b.viewLoading {
		rows = append(rows, StyleMuted.Render("  Loading…"))
	} else {
		rows = append(rows, b.viewViewport.View())
	}

	// Scroll hint
	pct := 0
	if b.viewViewport.TotalLineCount() > 0 {
		pct = int(b.viewViewport.ScrollPercent() * 100)
	}
	rows = append(rows, "", sep,
		StyleMuted.Render(fmt.Sprintf("  ⌫:back to directory  ↑/↓:scroll  %d%%", pct)),
	)

	return strings.Join(rows, "\n")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
