package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	versassh "github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/selfupdate"
)

// logCapture is an io.Writer that forwards each write to a channel.
type logCapture struct{ ch chan string }

func (lc *logCapture) Write(p []byte) (int, error) {
	lc.ch <- string(p)
	return len(p), nil
}

type msgDeployLogLine struct{ line string }
type msgDeployDone struct{ err error }
type msgRollbackDone struct{ err error }

// deployFlag is a toggleable deploy option shown in the Operations panel.
type deployFlag struct {
	label   string
	desc    string
	enabled bool
}

type operationsModel struct {
	// viewport for deploy log output
	viewport      viewport.Model
	logBuf        *strings.Builder
	logCh         chan string
	running       bool
	done          bool
	err           error
	status        string
	userScrolledUp bool

	logFilePath    string
	editingLogFile bool
	logFileInput   textinput.Model
	debugMode      bool

	// cursor navigates deploy options (0..5: 0-4 are bool flags, 5 is log file path)
	cursor int

	flags            []deployFlag
	deployLockExists bool
}

func newOperationsModel() operationsModel {
	li := textinput.New()
	li.Placeholder = "/path/to/deploy.log (optional)"
	li.CharLimit = 255

	return operationsModel{
		logBuf:       &strings.Builder{},
		logCh:        make(chan string, 256),
		logFileInput: li,
		flags: []deployFlag{
			{"Dry run", "Preview changes only — nothing is deployed", false},
			{"Force redeploy", "Redeploy even when no file changes are detected", false},
			{"Initial deploy", "First-time deployment — skips deploy.lock check", false},
			{"Skip dirty check", "Skip uncommitted changes validation", false},
			{"Debug mode", "Show verbose debug output in logs", false},
		},
	}
}

func (o *operationsModel) initViewport(width, height int) {
	vH := height - 10 // reserve rows for header + status + footer
	if vH < 4 {
		vH = 4
	}
	o.viewport = viewport.New(width-4, vH)
}

func (o *operationsModel) moveUp() {
	if o.cursor > 0 {
		o.cursor--
	}
}

func (o *operationsModel) moveDown() {
	// cursor max is len(o.flags) which is the log file path row (index 5)
	if o.cursor < len(o.flags) {
		o.cursor++
	}
}

func (o *operationsModel) toggleOption() {
	if o.cursor >= len(o.flags) {
		// cursor is on the log file path row — enter edit mode
		if o.cursor == len(o.flags) {
			o.enterLogFileEdit()
		}
		return
	}
	// Block toggling "Initial deploy" when deploy.lock exists on server
	if o.cursor == 2 && o.deployLockExists {
		return
	}
	o.flags[o.cursor].enabled = !o.flags[o.cursor].enabled
}

func (o *operationsModel) deployRunning() bool    { return o.running }
func (o operationsModel) dryRunVal() bool         { return o.flags[0].enabled }
func (o operationsModel) forceVal() bool          { return o.flags[1].enabled }
func (o operationsModel) initialDeployVal() bool  { return o.flags[2].enabled }
func (o operationsModel) skipDirtyCheckVal() bool { return o.flags[3].enabled }
func (o operationsModel) debugModeVal() bool      { return o.flags[4].enabled }

func (o *operationsModel) enterLogFileEdit() {
	o.editingLogFile = true
	o.logFileInput.SetValue(o.logFilePath)
	o.logFileInput.Focus()
}

func (o *operationsModel) confirmLogFile() {
	o.logFilePath = strings.TrimSpace(o.logFileInput.Value())
	o.editingLogFile = false
	o.logFileInput.Blur()
}

func (o *operationsModel) cancelLogFileEdit() {
	o.editingLogFile = false
	o.logFileInput.Blur()
}

func (o *operationsModel) clearLog() {
	o.running = false
	o.done = false
	o.err = nil
	o.status = ""
	o.logBuf = &strings.Builder{}
	o.viewport.SetContent("")
	o.userScrolledUp = false
}

func (o *operationsModel) startDeploy() {
	o.running = true
	o.done = false
	o.err = nil
	o.logBuf = &strings.Builder{}
	o.status = ""
	o.logCh = make(chan string, 256)
	o.viewport.SetContent("")
	o.userScrolledUp = false
}

func (o *operationsModel) appendLog(chunk string) {
	o.logBuf.WriteString(chunk)
	o.viewport.SetContent(o.logBuf.String())
	if !o.userScrolledUp {
		o.viewport.GotoBottom()
	}
}

func (o *operationsModel) scrollUp() {
	o.viewport.LineUp(3)
	o.userScrolledUp = o.viewport.ScrollPercent() < 1.0
}

func (o *operationsModel) scrollDown() {
	o.viewport.LineDown(3)
	o.userScrolledUp = o.viewport.ScrollPercent() < 1.0
}

func (o *operationsModel) scrollPageUp() {
	o.viewport.ViewUp()
	o.userScrolledUp = true
}

func (o *operationsModel) scrollPageDown() {
	o.viewport.ViewDown()
	o.userScrolledUp = o.viewport.ScrollPercent() < 1.0
}

// startDeploy launches the deployer in a goroutine and returns the first log tea.Cmd.
func startDeploy(cfg *config.Config, envName, repoPath string, dryRun, force, initialDeploy, skipDirtyCheck, debug bool, logFilePath string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var w io.Writer = &logCapture{ch: ch}
			if logFilePath != "" {
				f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err == nil {
					w = io.MultiWriter(&logCapture{ch: ch}, f)
					defer f.Close()
				}
			}
			log := logger.NewTUILogger(w, true, debug)
			d, err := deployer.NewDeployer(cfg, envName, repoPath, dryRun, initialDeploy, force, skipDirtyCheck, log)
			if err != nil {
				ch <- fmt.Sprintf("[ERROR] %v\n", err)
				close(ch)
				return
			}
			if err = d.Deploy(); err != nil {
				ch <- fmt.Sprintf("[ERROR] %v\n", err)
			}
			close(ch)
		}()
		return waitForLogLine(ch)()
	}
}

// waitForLogLine returns a tea.Cmd that blocks for the next log line or channel close.
func waitForLogLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgDeployDone{}
		}
		return msgDeployLogLine{line: line}
	}
}

// Quick action commands

func doSSHTest(cfg *config.Config, envName string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			env, err := cfg.GetEnvironment(envName)
			if err != nil {
				ch <- fmt.Sprintf("[ERROR] %v\n", err)
				close(ch)
				return
			}
			log := logger.NewTUILogger(&logCapture{ch: ch}, true, false)
			client, err := versassh.NewClient(&env.SSH, log)
			if err != nil {
				ch <- fmt.Sprintf("[ERROR] SSH connection failed: %v\n", err)
				close(ch)
				return
			}
			defer client.Close()
			ch <- "[INFO] SSH connection established\n"
			out, _ := client.ExecuteCommand("uname -a")
			if out != "" {
				ch <- fmt.Sprintf("[INFO] Remote: %s\n", strings.TrimSpace(out))
			}
			ch <- "[✓] SSH test passed\n"
			close(ch)
		}()
		return waitForLogLine(ch)()
	}
}

func doSelfUpdate(ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			log := logger.NewTUILogger(&logCapture{ch: ch}, true, false)
			updater := selfupdate.NewUpdater(log)
			if err := updater.Update(); err != nil {
				ch <- fmt.Sprintf("[ERROR] Self-update failed: %v\n", err)
			} else {
				ch <- "[✓] Self-update completed\n"
			}
			close(ch)
		}()
		return waitForLogLine(ch)()
	}
}

func doStatus(client *versassh.Client, remotePath string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
			target, err := client.ReadSymlink(currentSymlink)
			if err != nil {
				ch <- fmt.Sprintf("[WARN] No current symlink: %v\n", err)
			} else {
				ch <- fmt.Sprintf("[INFO] Current release: %s\n", filepath.Base(target))
			}
			releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
			releases, err := client.ListReleases(releasesDir)
			if err != nil {
				ch <- fmt.Sprintf("[WARN] Could not list releases: %v\n", err)
			} else {
				ch <- fmt.Sprintf("[INFO] Total releases: %d\n", len(releases))
			}
			close(ch)
		}()
		return waitForLogLine(ch)()
	}
}

// doRollback rolls back to the explicitly named release.
func doRollback(client *versassh.Client, remotePath, targetRelease string) tea.Cmd {
	return func() tea.Msg {
		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		relTarget := filepath.ToSlash(filepath.Join("releases", targetRelease))
		err := client.CreateSymlink(relTarget, currentSymlink)
		return msgRollbackDone{err: err}
	}
}

// doRollbackToPrevious rolls back to the release immediately before the current one.
func doRollbackToPrevious(client *versassh.Client, remotePath string) tea.Cmd {
	return func() tea.Msg {
		releasesDir := filepath.ToSlash(filepath.Join(remotePath, "releases"))
		releases, err := client.ListReleases(releasesDir)
		if err != nil {
			return msgRollbackDone{err: fmt.Errorf("could not list releases: %w", err)}
		}
		if len(releases) < 2 {
			return msgRollbackDone{err: fmt.Errorf("no previous release to rollback to")}
		}

		currentSymlink := filepath.ToSlash(filepath.Join(remotePath, "current"))
		currentTarget, _ := client.ReadSymlink(currentSymlink)
		currentRelease := filepath.Base(currentTarget)

		// Sort newest first
		sortReleases(releases)

		var previous string
		for _, r := range releases {
			if r != currentRelease {
				previous = r
				break
			}
		}
		if previous == "" {
			return msgRollbackDone{err: fmt.Errorf("could not determine previous release")}
		}

		relTarget := filepath.ToSlash(filepath.Join("releases", previous))
		err = client.CreateSymlink(relTarget, currentSymlink)
		return msgRollbackDone{err: err}
	}
}

func sortReleases(releases []string) {
	// Simple descending string sort — works because format is YYYYMMDD-HHMMSS
	for i := 0; i < len(releases)-1; i++ {
		for j := i + 1; j < len(releases); j++ {
			if releases[j] > releases[i] {
				releases[i], releases[j] = releases[j], releases[i]
			}
		}
	}
}

func (o operationsModel) view(width, height int, currentRelease string) string {
	// When running or showing log output: compact header + full-width log viewport
	if o.running || o.done || o.logBuf.Len() > 0 {
		return o.viewRunning(width, currentRelease)
	}
	return o.viewIdle(width, currentRelease)
}

// viewIdle renders the control panel (deploy options, rollback, quick actions).
func (o operationsModel) viewIdle(width int, currentRelease string) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	rows := []string{"", StyleTitle.Render("  Operations"), ""}

	// ── DEPLOY SECTION ─────────────────────────────
	rows = append(rows,
		StyleSection.Render("  ▸  Deploy"),
		StyleHint.Render("     Deploy current repository state to the remote server"),
		"",
	)

	for i, f := range o.flags {
		check := StyleMuted.Render("[ ]")
		if f.enabled {
			check = StyleSuccess.Render("[\U000F012C]") // mdi-check-circle
		}
		label := f.label
		desc := f.desc
		if i == 2 && o.deployLockExists {
			label = f.label + StyleMuted.Render(" (deploy.lock exists)")
			check = StyleMuted.Render("[\U000F0159]") // mdi-close-circle
		}
		descRender := StyleHint.Render("  " + desc)
		line := fmt.Sprintf("  %s  %-30s%s", check, label, descRender)
		if i == o.cursor {
			line = StyleSelected.Render(fmt.Sprintf("  %s  %-30s", check, label)) + descRender
		}
		rows = append(rows, line)
	}

	// Log file path row (cursor index == len(o.flags) == 5)
	logFileIdx := len(o.flags)
	if o.editingLogFile {
		rows = append(rows, fmt.Sprintf("  \U0001F4C4 Log file path: %s", o.logFileInput.View()))
	} else {
		logVal := o.logFilePath
		if logVal == "" {
			logVal = StyleMuted.Render("(not set)")
		}
		logLine := fmt.Sprintf("  \U0001F4C4 Log file path: %s", logVal)
		if o.cursor == logFileIdx {
			logLine = StyleSelected.Render(fmt.Sprintf("  \U0001F4C4 Log file path: %-40s", logVal))
		}
		rows = append(rows, logLine)
	}

	rows = append(rows, "", fmt.Sprintf("  %s Start deploy", StyleCmd.Render("[D]")))

	// ── ROLLBACK SECTION ───────────────────────────
	rows = append(rows, "", sep, "", StyleSection.Render("  ◂  Rollback"))
	if currentRelease != "" {
		rows = append(rows, StyleHint.Render("     Current: "+currentRelease))
	}
	rows = append(rows,
		"",
		fmt.Sprintf("  %s Rollback to previous release", StyleCmd.Render("[R]")),
		StyleMuted.Render("     For specific version → view 2 (Releases)"),
	)

	// ── QUICK ACTIONS ──────────────────────────────
	rows = append(rows,
		"", sep, "",
		StyleSection.Render("  »  Quick Actions"),
		"",
		fmt.Sprintf("  %s SSH connection test", StyleCmd.Render("[s]")),
		fmt.Sprintf("  %s Check for updates", StyleCmd.Render("[u]")),
		fmt.Sprintf("  %s Show deployment status", StyleCmd.Render("[t]")),
	)

	if o.status != "" {
		rows = append(rows, "", "  "+o.status)
	}

	rows = append(rows, "", sep, "",
		StyleMuted.Render("  ↑/↓ navigate   ↵ toggle/edit   D deploy   R rollback   s/u/t quick actions"),
	)

	return strings.Join(rows, "\n")
}

// viewRunning renders the log viewport full-screen with a compact status bar at top.
func (o operationsModel) viewRunning(width int, currentRelease string) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	// Compact flag summary on one line
	flagSummary := ""
	for _, f := range o.flags {
		if f.enabled {
			if flagSummary != "" {
				flagSummary += "  "
			}
			flagSummary += StyleSuccess.Render("[\U000F012C]") + " " + f.label // mdi-check-circle
		}
	}
	if flagSummary == "" {
		flagSummary = StyleMuted.Render("default flags")
	}

	stateStr := StyleWarning.Render("\U000F0765 running…") // mdi-circle
	if o.done {
		if o.err != nil {
			stateStr = StyleError.Render("\U000F0159 failed") // mdi-close-circle
		} else {
			stateStr = StyleSuccess.Render("\U000F012C done") // mdi-check-circle
		}
	}

	rows := []string{
		"",
		StyleTitle.Render("  Operations") + "  " + stateStr,
		StyleHint.Render("  " + flagSummary),
		"",
		sep,
		"",
		StyleSection.Render("  Output log:"),
		"",
		o.viewport.View(),
		"",
		sep,
	}

	if o.done {
		if o.err != nil {
			rows = append(rows, StyleError.Render("  ✕ "+o.err.Error()))
		} else {
			rows = append(rows, StyleSuccess.Render("  ✓ Operation completed successfully"))
		}
		rows = append(rows, "", StyleMuted.Render("  Esc=close log   D=re-deploy  R=rollback  ←/→=navigate views   ↑↓:scroll  PgUp/PgDn:page"))
	} else {
		rows = append(rows, StyleMuted.Render("  Esc=close   ←/→=switch views   ↑↓:scroll  PgUp/PgDn:page   (running…)"))
	}

	return strings.Join(rows, "\n")
}
