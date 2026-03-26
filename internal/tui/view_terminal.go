package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

// Terminal messages
type msgTerminalOutput struct{ line string }
type msgTerminalDone struct{ err error }
type msgCdResolved struct {
	newCwd string
	oldCwd string
	err    error
}
type msgTabComplete struct {
	matches []string
	prefix  string // text before the completed token
	token   string // original partial token
	err     error
}

type terminalModel struct {
	input   textinput.Model
	vp      viewport.Model
	logBuf  *strings.Builder
	logCh   chan string
	doneCh  chan error
	running bool
	history []string
	histIdx int
	cwd     string // remote working directory

	// Tab completion state
	completions   []string
	completionIdx int
	completionOn  bool // true while cycling through completions
	completionPfx string // the prefix text before the token being completed
	completionTok string // the original token being completed
}

func newTerminalModel() terminalModel {
	ti := textinput.New()
	ti.Placeholder = "Type a command and press Enter..."
	ti.CharLimit = 1024
	ti.Focus()

	return terminalModel{
		input:  ti,
		logBuf: &strings.Builder{},
	}
}

func (t *terminalModel) initViewport(width, height int) {
	vpH := height - 6
	if vpH < 4 {
		vpH = 4
	}
	t.vp = viewport.New(width-4, vpH)
}

func (t *terminalModel) appendOutput(chunk string) {
	t.logBuf.WriteString(chunk)
	t.vp.SetContent(t.logBuf.String())
	t.vp.GotoBottom()
}

func (t *terminalModel) executeCommand(client *versassh.Client, cmd string) tea.Cmd {
	t.running = true
	t.logBuf.WriteString(fmt.Sprintf("%s $ %s\n", t.cwd, cmd))
	t.vp.SetContent(t.logBuf.String())
	t.vp.GotoBottom()

	// Save to history
	t.history = append(t.history, cmd)
	t.histIdx = len(t.history)

	t.logCh = make(chan string, 256)
	t.doneCh = make(chan error, 1)

	// Wrap command with cd to cwd so it runs in the project directory
	wrappedCmd := fmt.Sprintf("cd %s && %s", t.cwd, cmd)

	return func() tea.Msg {
		go func() {
			w := &termWriter{ch: t.logCh}
			err := client.ExecuteCommandStreaming(wrappedCmd, w, w)
			t.doneCh <- err
			close(t.logCh)
		}()
		return waitForTermLine(t.logCh)()
	}
}

func (t *terminalModel) historyUp() {
	if len(t.history) == 0 {
		return
	}
	if t.histIdx > 0 {
		t.histIdx--
	}
	t.input.SetValue(t.history[t.histIdx])
}

func (t *terminalModel) historyDown() {
	if len(t.history) == 0 {
		return
	}
	if t.histIdx < len(t.history)-1 {
		t.histIdx++
		t.input.SetValue(t.history[t.histIdx])
	} else {
		t.histIdx = len(t.history)
		t.input.SetValue("")
	}
}

func (t *terminalModel) resetCompletion() {
	t.completions = nil
	t.completionIdx = 0
	t.completionOn = false
	t.completionPfx = ""
	t.completionTok = ""
}

// termWriter forwards writes to a channel line by line.
type termWriter struct{ ch chan string }

func (w *termWriter) Write(p []byte) (int, error) {
	w.ch <- string(p)
	return len(p), nil
}

func waitForTermLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgTerminalDone{}
		}
		return msgTerminalOutput{line: line}
	}
}

func waitForTermDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-ch
		return msgTerminalDone{err: err}
	}
}

func (t terminalModel) view(width, height int) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	cwdDisplay := t.cwd
	if cwdDisplay == "" {
		cwdDisplay = "~"
	}

	rows := []string{
		"",
		StyleTitle.Render("  Remote Terminal"),
		StyleHint.Render(fmt.Sprintf("  Working directory: %s", cwdDisplay)),
		"",
		sep,
		"",
	}

	// Viewport with output
	rows = append(rows, t.vp.View())
	rows = append(rows, "", sep)

	// Input line
	if t.running {
		rows = append(rows, StyleWarning.Render("  ⏳ Running..."))
	} else {
		rows = append(rows, fmt.Sprintf("  %s %s", StyleCmd.Render(cwdDisplay+" $"), t.input.View()))
	}

	rows = append(rows, "",
		StyleMuted.Render("  ↵:execute  ↑/↓:history  Tab:complete  PgUp/PgDn/scroll  Ctrl+L:clear  Esc:back"),
	)

	return strings.Join(rows, "\n")
}

// resolveCd runs the cd command on the server and returns the resolved absolute path.
func resolveCd(client *versassh.Client, resolveCmd, oldCwd string) tea.Cmd {
	return func() tea.Msg {
		output, err := client.ExecuteCommand(resolveCmd)
		if err != nil {
			return msgCdResolved{oldCwd: oldCwd, err: err}
		}
		newPath := strings.TrimSpace(output)
		if newPath == "" {
			return msgCdResolved{oldCwd: oldCwd, err: fmt.Errorf("could not resolve directory")}
		}
		return msgCdResolved{newCwd: newPath, oldCwd: oldCwd}
	}
}

// tabComplete lists remote files/dirs matching the partial path in the input.
func tabComplete(client *versassh.Client, cwd, prefix, token string) tea.Cmd {
	return func() tea.Msg {
		// Determine the directory to list and the partial name to match
		dir := cwd
		partial := token

		if idx := strings.LastIndex(token, "/"); idx >= 0 {
			pathPart := token[:idx+1]
			partial = token[idx+1:]
			if strings.HasPrefix(pathPart, "/") {
				dir = pathPart
			} else {
				dir = cwd + "/" + pathPart
			}
		}

		// List files in the target directory
		lsCmd := fmt.Sprintf("ls -1Ap %s 2>/dev/null", dir)
		output, err := client.ExecuteCommand(lsCmd)
		if err != nil {
			return msgTabComplete{err: err, prefix: prefix, token: token}
		}

		var matches []string
		for _, entry := range strings.Split(strings.TrimSpace(output), "\n") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if partial == "" || strings.HasPrefix(entry, partial) {
				// Rebuild the full token with the match
				if idx := strings.LastIndex(token, "/"); idx >= 0 {
					matches = append(matches, token[:idx+1]+entry)
				} else {
					matches = append(matches, entry)
				}
			}
		}

		return msgTabComplete{matches: matches, prefix: prefix, token: token}
	}
}
