package tui

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
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

type msgXferLine struct{ line string }
type msgXferStreamEnd struct{}
type msgXferDone struct{ err error }

type msgDeleteDone struct {
	path string
	err  error
}

// ── Local file browser ────────────────────────────────────────────────────────

type localBrowser struct {
	stack     []string
	entries   []os.FileInfo
	cursor    int
	viewStart int
	selected  map[string]bool // absolute path → selected
	err       error
}

func (lb *localBrowser) init(startPath string) {
	lb.stack = []string{startPath}
	lb.selected = make(map[string]bool)
	lb.cursor = 0
	lb.viewStart = 0
	lb.loadEntries()
}

func (lb *localBrowser) currentPath() string {
	if len(lb.stack) == 0 {
		return "/"
	}
	return lb.stack[len(lb.stack)-1]
}

func (lb *localBrowser) hasParent() bool { return len(lb.stack) > 1 }

func (lb *localBrowser) loadEntries() {
	dirEntries, err := os.ReadDir(lb.currentPath())
	if err != nil {
		lb.err = err
		lb.entries = nil
		return
	}
	lb.err = nil
	lb.entries = nil
	for _, e := range dirEntries {
		info, err := e.Info()
		if err == nil {
			lb.entries = append(lb.entries, info)
		}
	}
}

func (lb *localBrowser) totalVisible() int {
	n := len(lb.entries)
	if lb.hasParent() {
		n++
	}
	return n
}

func (lb *localBrowser) entryAt(pos int) os.FileInfo {
	if lb.hasParent() {
		if pos == 0 {
			return nil // ".." virtual entry
		}
		return lb.entries[pos-1]
	}
	return lb.entries[pos]
}

func (lb *localBrowser) moveUp() {
	if lb.cursor > 0 {
		lb.cursor--
	}
}

func (lb *localBrowser) moveDown() {
	if lb.cursor < lb.totalVisible()-1 {
		lb.cursor++
	}
}

// enterSelected: navigate into dir, or toggle file selection.
func (lb *localBrowser) enterSelected() {
	entry := lb.entryAt(lb.cursor)
	if entry == nil {
		lb.popDir()
		return
	}
	fullPath := filepath.Join(lb.currentPath(), entry.Name())
	if entry.IsDir() {
		lb.stack = append(lb.stack, fullPath)
		lb.cursor = 0
		lb.viewStart = 0
		lb.loadEntries()
	} else {
		lb.toggleSelect(fullPath)
	}
}

func (lb *localBrowser) spaceSelect() {
	entry := lb.entryAt(lb.cursor)
	if entry == nil || entry.IsDir() {
		return
	}
	lb.toggleSelect(filepath.Join(lb.currentPath(), entry.Name()))
}

func (lb *localBrowser) toggleSelect(path string) {
	if lb.selected[path] {
		delete(lb.selected, path)
	} else {
		lb.selected[path] = true
	}
}

func (lb *localBrowser) popDir() {
	if len(lb.stack) > 1 {
		lb.stack = lb.stack[:len(lb.stack)-1]
		lb.cursor = 0
		lb.viewStart = 0
		lb.loadEntries()
	}
}

func (lb *localBrowser) selectedFiles() []string {
	files := make([]string, 0, len(lb.selected))
	for path := range lb.selected {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}

// ── Transfer state ─────────────────────────────────────────────────────────────

type xferDir int

const (
	xferNone     xferDir = iota
	xferDownload         // server → local
	xferUpload           // local → server
)

type xferState struct {
	dir        xferDir
	sshCfg     *config.SSHConfig
	sshUser    string
	sshHost    string
	remotePath string // upload: dest dir on server; download: source file on server

	// Navigable local filesystem browser (used for both upload and download)
	local localBrowser

	// Transfer execution
	logLines []string
	running  bool
	done     bool
	err      error
	logCh    chan string
	doneCh   chan error
}

// ── Browser model ─────────────────────────────────────────────────────────────

type browserModel struct {
	stack     []string
	entries   []os.FileInfo
	cursor    int
	viewStart int
	loaded    bool
	err       error

	viewing         bool
	viewPath        string
	viewInfo        os.FileInfo
	viewContent     []byte
	viewLoading     bool
	viewErr         error
	viewViewport    viewport.Model
	viewInitialized bool

	statusMsg string // inline status (delete result, etc.)
	xfer      xferState
}

// ── SSH / rsync commands ───────────────────────────────────────────────────────

func listDir(client *versassh.Client, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.ReadDir(path)
		return msgDirListed{path: path, entries: entries, err: err}
	}
}

func loadFileContent(client *versassh.Client, path string, info os.FileInfo) tea.Cmd {
	return func() tea.Msg {
		const maxRead = 512 * 1024
		data, err := client.ReadRemoteBytes(path, maxRead)
		return msgFileContent{path: path, content: data, info: info, err: err}
	}
}

func buildRsyncArgs(sshCfg *config.SSHConfig, srcs []string, dst string) []string {
	port := sshCfg.Port
	if port == 0 {
		port = 22
	}
	sshArg := fmt.Sprintf("ssh -p %d", port)
	if sshCfg.KeyPath != "" {
		sshArg += " -i " + sshCfg.KeyPath
	}
	args := []string{"-avz", "--info=progress2", "-e", sshArg}
	args = append(args, srcs...)
	args = append(args, dst)
	return args
}

func startRsyncCmd(sshCfg *config.SSHConfig, srcs []string, dst string, logCh chan string, doneCh chan error) tea.Cmd {
	return func() tea.Msg {
		go func() {
			pr, pw := io.Pipe()
			cmd := exec.Command("rsync", buildRsyncArgs(sshCfg, srcs, dst)...)
			cmd.Stdout = pw
			cmd.Stderr = pw
			if err := cmd.Start(); err != nil {
				_ = pw.Close()
				_ = pr.Close()
				logCh <- "error starting rsync: " + err.Error()
				close(logCh)
				doneCh <- err
				return
			}
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				logCh <- scanner.Text()
			}
			_ = pr.Close()
			waitErr := cmd.Wait()
			close(logCh)
			doneCh <- waitErr
		}()
		return waitForXferLine(logCh)()
	}
}

func waitForXferLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgXferStreamEnd{}
		}
		return msgXferLine{line: line}
	}
}

func waitForXferDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		return msgXferDone{err: <-ch}
	}
}

func deleteFileCmd(client *versassh.Client, path string) tea.Cmd {
	return func() tea.Msg {
		return msgDeleteDone{path: path, err: client.Remove(path)}
	}
}

// ── Browser model methods ─────────────────────────────────────────────────────

func (b *browserModel) init(remotePath string) {
	b.stack = []string{remotePath}
	b.entries = nil
	b.cursor = 0
	b.viewStart = 0
	b.loaded = false
	b.viewing = false
	b.xfer = xferState{}
}

func (b *browserModel) initDownload(remotePath, sshUser, sshHost string, sshCfg *config.SSHConfig) {
	startDir, _ := os.UserHomeDir()
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	var lb localBrowser
	lb.init(startDir)
	b.xfer = xferState{
		dir:        xferDownload,
		sshCfg:     sshCfg,
		sshUser:    sshUser,
		sshHost:    sshHost,
		remotePath: remotePath,
		local:      lb,
		logCh:      make(chan string, 128),
		doneCh:     make(chan error, 1),
	}
}

func (b *browserModel) initUpload(remoteDir, sshUser, sshHost string, sshCfg *config.SSHConfig) {
	startDir, _ := os.UserHomeDir()
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	var lb localBrowser
	lb.init(startDir)
	b.xfer = xferState{
		dir:        xferUpload,
		sshCfg:     sshCfg,
		sshUser:    sshUser,
		sshHost:    sshHost,
		remotePath: remoteDir,
		local:      lb,
		logCh:      make(chan string, 128),
		doneCh:     make(chan error, 1),
	}
}

func (b *browserModel) cancelXfer() {
	b.xfer = xferState{}
}

func (b *browserModel) appendXferLine(line string) {
	const maxLines = 80
	b.xfer.logLines = append(b.xfer.logLines, line)
	if len(b.xfer.logLines) > maxLines {
		b.xfer.logLines = b.xfer.logLines[len(b.xfer.logLines)-maxLines:]
	}
}

func (b *browserModel) applyListed(msg msgDirListed) {
	b.entries = msg.entries
	b.err = msg.err
	b.loaded = true
	b.cursor = 0
	b.viewStart = 0
	b.viewing = false
	b.statusMsg = ""
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

func (b *browserModel) totalVisible() int {
	n := len(b.entries)
	if b.hasParent() {
		n++
	}
	return n
}

func (b *browserModel) entryAt(pos int) os.FileInfo {
	if b.hasParent() {
		if pos == 0 {
			return nil
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

func (b *browserModel) enterSelected() (dirPath string, filePath string, info os.FileInfo) {
	if b.totalVisible() == 0 {
		return
	}
	entry := b.entryAt(b.cursor)
	if entry == nil {
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

func (b *browserModel) selectedRemoteFile() string {
	if b.totalVisible() == 0 {
		return ""
	}
	entry := b.entryAt(b.cursor)
	if entry == nil || entry.IsDir() {
		return ""
	}
	return filepath.ToSlash(filepath.Join(b.currentPath(), entry.Name()))
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
		var hexParts []string
		for j := 0; j < len(h); j += 2 {
			hexParts = append(hexParts, h[j:j+2])
		}
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

func (b browserModel) view(width, height int) string {
	if b.xfer.dir != xferNone {
		return b.xferView(width, height)
	}
	if b.viewing {
		return b.viewFileView(width)
	}
	return b.dirView(width, height)
}

// ── Transfer panel ────────────────────────────────────────────────────────────

func (b browserModel) xferView(width, height int) string {
	// While running or done: show log output
	if b.xfer.running || b.xfer.done {
		return b.xferLogView(width)
	}
	// Pre-transfer: show local browser (upload) or destination input (download)
	if b.xfer.dir == xferUpload {
		return b.xferUploadBrowserView(width, height)
	}
	return b.xferDownloadView(width, height)
}

// xferUploadBrowserView renders the local file browser for selecting files to upload.
func (b browserModel) xferUploadBrowserView(width, height int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	lb := &b.xfer.local

	remoteLabel := fmt.Sprintf("%s@%s:%s", b.xfer.sshUser, b.xfer.sshHost, b.xfer.remotePath)
	rows := []string{
		"",
		StyleTitle.Render("  ↑  Upload  —  local → server"),
		"",
		StyleMuted.Render("  To:  ") + StyleActive.Render(remoteLabel),
		"",
		StyleMuted.Render("  Local: ") + StyleSection.Render(lb.currentPath()),
		sep,
	}

	if lb.err != nil {
		rows = append(rows, StyleError.Render("  "+lb.err.Error()))
	} else {
		total := lb.totalVisible()
		if total == 0 {
			rows = append(rows, StyleMuted.Render("  (empty directory)"))
		} else {
			// header(7) + sep(1) + selected summary(~3) + sep(1) + footer(2) = ~14 overhead
			vH := height - 14
			if vH < 3 {
				vH = 3
			}
			if vH > total {
				vH = total
			}
			// Scroll
			if lb.cursor >= lb.viewStart+vH {
				lb.viewStart = lb.cursor - vH + 1
			} else if lb.cursor < lb.viewStart {
				lb.viewStart = lb.cursor
			}
			if lb.viewStart < 0 {
				lb.viewStart = 0
			}
			endIdx := lb.viewStart + vH
			if endIdx > total {
				endIdx = total
			}

			for i := lb.viewStart; i < endIdx; i++ {
				entry := lb.entryAt(i)
				var line string
				if entry == nil {
					icon := " " + iconParentDir()
					if i == lb.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, ".."))
					} else {
						line = StyleMuted.Render(fmt.Sprintf("  %s%s", icon, ".."))
					}
				} else {
					icon := " " + iconForEntry(entry.Name(), entry.IsDir())
					name := entry.Name()
					if utf8.RuneCountInString(name) > 28 {
						runes := []rune(name)
						name = string(runes[:25]) + "..."
					}

					// selection marker (files only)
					marker := "  "
					absPath := filepath.Join(lb.currentPath(), entry.Name())
					if !entry.IsDir() && lb.selected[absPath] {
						marker = StyleSuccess.Render("✓ ")
					}

					size := ""
					if !entry.IsDir() {
						size = StyleMuted.Render(fmt.Sprintf("%9s", humanSize(entry.Size())))
					} else {
						size = strings.Repeat(" ", 9)
					}
					modTime := StyleMuted.Render(entry.ModTime().Format(" 2006-01-02 15:04"))

					if i == lb.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%s%-28s", marker, icon, name)) +
							size + modTime
					} else {
						line = fmt.Sprintf("  %s%s%-28s%s%s", marker, icon, name, size, modTime)
					}
				}
				rows = append(rows, line)
			}

			if total > vH {
				pct := int((float64(lb.cursor) / float64(total-1)) * 100)
				rows = append(rows, StyleMuted.Render(fmt.Sprintf(
					"  … %d more  (%d%%)", total-endIdx, pct)))
			}
		}
	}

	rows = append(rows, "", sep, "")

	// Selected files summary
	sel := lb.selectedFiles()
	if len(sel) == 0 {
		rows = append(rows, StyleMuted.Render("  No files selected"))
	} else {
		rows = append(rows, StyleSuccess.Render(fmt.Sprintf("  %d file(s) selected:", len(sel))))
		for _, f := range sel {
			name := filepath.Base(f)
			if len(name) > width-8 {
				name = "…" + name[len(name)-(width-9):]
			}
			rows = append(rows, StyleMuted.Render("    • "+name))
		}
	}

	rows = append(rows, "", sep, "")

	if len(sel) > 0 {
		rows = append(rows,
			StyleMuted.Render("  ↑/↓:move  Enter:dir/select  Space:select  ⌫:up"),
			StyleCmd.Render("  t")+StyleMuted.Render(":start upload   Esc:cancel"),
		)
	} else {
		rows = append(rows,
			StyleMuted.Render("  ↑/↓:move  Enter:open dir / select file  Space:select file  ⌫:up  Esc:cancel"),
		)
	}

	return strings.Join(rows, "\n")
}

// xferDownloadView renders the local directory browser for choosing a download destination.
func (b browserModel) xferDownloadView(width, height int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	lb := &b.xfer.local

	remoteLabel := fmt.Sprintf("%s@%s:%s", b.xfer.sshUser, b.xfer.sshHost, b.xfer.remotePath)
	rows := []string{
		"",
		StyleTitle.Render("  ↓  Download  —  server → local"),
		"",
		StyleMuted.Render("  From: ") + StyleActive.Render(remoteLabel),
		"",
		StyleMuted.Render("  Save to: ") + StyleSection.Render(lb.currentPath()),
		sep,
	}

	if lb.err != nil {
		rows = append(rows, StyleError.Render("  "+lb.err.Error()))
	} else {
		total := lb.totalVisible()
		if total == 0 {
			rows = append(rows, StyleMuted.Render("  (empty directory)"))
		} else {
			// header(7) + sep(1) + footer(2) = ~10 overhead
			vH := height - 10
			if vH < 3 {
				vH = 3
			}
			if vH > total {
				vH = total
			}
			if lb.cursor >= lb.viewStart+vH {
				lb.viewStart = lb.cursor - vH + 1
			} else if lb.cursor < lb.viewStart {
				lb.viewStart = lb.cursor
			}
			if lb.viewStart < 0 {
				lb.viewStart = 0
			}
			endIdx := lb.viewStart + vH
			if endIdx > total {
				endIdx = total
			}

			for i := lb.viewStart; i < endIdx; i++ {
				entry := lb.entryAt(i)
				var line string
				if entry == nil {
					icon := " " + iconParentDir()
					if i == lb.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, ".."))
					} else {
						line = StyleMuted.Render(fmt.Sprintf("  %s%s", icon, ".."))
					}
				} else {
					icon := " " + iconForEntry(entry.Name(), entry.IsDir())
					name := entry.Name()
					if utf8.RuneCountInString(name) > 28 {
						runes := []rune(name)
						name = string(runes[:25]) + "..."
					}
					modTime := StyleMuted.Render(entry.ModTime().Format(" 2006-01-02 15:04"))
					size := strings.Repeat(" ", 9)
					if !entry.IsDir() {
						size = StyleMuted.Render(fmt.Sprintf("%9s", humanSize(entry.Size())))
					}
					if i == lb.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, name)) +
							size + modTime
					} else {
						line = fmt.Sprintf("  %s%-30s%s%s", icon, name, size, modTime)
					}
				}
				rows = append(rows, line)
			}

			if total > vH {
				pct := int((float64(lb.cursor) / float64(total-1)) * 100)
				rows = append(rows, StyleMuted.Render(fmt.Sprintf(
					"  … %d more  (%d%%)", total-endIdx, pct)))
			}
		}
	}

	rows = append(rows, "", sep, "",
		StyleMuted.Render("  ↑/↓:move  Enter:open dir  ⌫:up"),
		StyleCmd.Render("  t")+StyleMuted.Render(":download here   Esc:cancel"),
	)
	return strings.Join(rows, "\n")
}

// xferLogView renders the rsync output log while running or after completion.
func (b browserModel) xferLogView(width int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	var dirLabel string
	if b.xfer.dir == xferDownload {
		dirLabel = "↓  Download"
	} else {
		dirLabel = "↑  Upload"
	}

	stateStr := StyleWarning.Render("  ● Running…")
	if b.xfer.done {
		if b.xfer.err != nil {
			stateStr = StyleError.Render("  ✕ Failed: " + b.xfer.err.Error())
		} else {
			stateStr = StyleSuccess.Render("  ✓ Transfer complete")
		}
	}

	rows := []string{
		"",
		StyleTitle.Render("  " + dirLabel),
		"",
		stateStr,
		"",
		StyleMuted.Render("  ── rsync output " + strings.Repeat("─", max(width-22, 4))),
		"",
	}

	if len(b.xfer.logLines) == 0 {
		rows = append(rows, StyleMuted.Render("  Starting…"))
	} else {
		for _, l := range b.xfer.logLines {
			rows = append(rows, "  "+l)
		}
	}

	rows = append(rows, "", sep, "")

	if b.xfer.done {
		rows = append(rows, StyleMuted.Render("  Esc=back   Enter=close"))
	} else {
		rows = append(rows, StyleMuted.Render("  Esc=back   (rsync running…)"))
	}

	return strings.Join(rows, "\n")
}

// ── Remote directory view ─────────────────────────────────────────────────────

func (b browserModel) dirView(width, height int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	title := StyleTitle.Render("  File Browser")
	pathLine := StyleMuted.Render("  Path: ") + StyleActive.Render(b.currentPath())

	rows := []string{"", title, "", pathLine, "", sep}

	if !b.loaded {
		rows = append(rows, "", StyleMuted.Render("  Loading…"))
	} else if b.err != nil {
		rows = append(rows, "", StyleError.Render("  Error: "+b.err.Error()))
	} else {
		total := b.totalVisible()
		if total == 0 {
			rows = append(rows, "", StyleMuted.Render("  (empty directory)"))
		} else {
			// header(6) + footer(3) = ~9 overhead; use height-based window
			vH := height - 9
			if vH < 3 {
				vH = 3
			}
			if vH > total {
				vH = total
			}
			if b.cursor >= b.viewStart+vH {
				b.viewStart = b.cursor - vH + 1
			} else if b.cursor < b.viewStart {
				b.viewStart = b.cursor
			}
			if b.viewStart < 0 {
				b.viewStart = 0
			}
			endIdx := b.viewStart + vH
			if endIdx > total {
				endIdx = total
			}

			for i := b.viewStart; i < endIdx; i++ {
				entry := b.entryAt(i)
				var line string
				if entry == nil {
					icon := " " + iconParentDir()
					if i == b.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, ".."))
					} else {
						line = StyleMuted.Render(fmt.Sprintf("  %s%s", icon, ".."))
					}
				} else {
					icon := " " + iconForEntry(entry.Name(), entry.IsDir())
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
					modTime := StyleMuted.Render(entry.ModTime().Format(" 2006-01-02 15:04"))
					if i == b.cursor {
						line = StyleSelected.Render(fmt.Sprintf("  %s%-30s", icon, name)) +
							size + modTime
					} else {
						line = fmt.Sprintf("%s%-30s%s%s", icon, name, size, modTime)
					}
				}
				rows = append(rows, line)
			}

			if total > vH {
				pct := int((float64(b.cursor) / float64(total-1)) * 100)
				rows = append(rows, StyleMuted.Render(fmt.Sprintf(
					"  … %d more  (%d%%)", total-endIdx, pct)))
			}
		}
	}

	if b.statusMsg != "" {
		rows = append(rows, "", "  "+b.statusMsg)
	}
	rows = append(rows, "", sep, "",
		StyleMuted.Render("  ↵:open  ⌫:up  ↑/↓:navigate  d:download  u:upload  Del:delete"),
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

	pct := 0
	if b.viewViewport.TotalLineCount() > 0 {
		pct = int(b.viewViewport.ScrollPercent() * 100)
	}
	rows = append(rows, "", sep,
		StyleMuted.Render(fmt.Sprintf("  Esc:back  ⌫:back  ↑/↓:scroll  d:download  %d%%", pct)),
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
