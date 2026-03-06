package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	versassh "github.com/user/versaDeploy/internal/ssh"
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
	viewport viewport.Model
	logBuf   strings.Builder
	logCh    chan string
	running  bool
	done     bool
	err      error
	status   string

	// cursor navigates deploy options (0..2)
	cursor int

	flags []deployFlag
}

func newOperationsModel() operationsModel {
	return operationsModel{
		logCh: make(chan string, 256),
		flags: []deployFlag{
			{"Dry run", "Preview changes only — nothing is deployed", false},
			{"Force redeploy", "Redeploy even when no file changes are detected", false},
			{"Initial deploy", "First-time deployment — skips deploy.lock check", false},
		},
	}
}

func (o *operationsModel) initViewport(width, height int) {
	vH := height - 22 // reserve rows for options + headers
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
	if o.cursor < len(o.flags)-1 {
		o.cursor++
	}
}

func (o *operationsModel) toggleOption() {
	if o.cursor < len(o.flags) {
		o.flags[o.cursor].enabled = !o.flags[o.cursor].enabled
	}
}

func (o *operationsModel) deployRunning() bool   { return o.running }
func (o operationsModel) dryRunVal() bool        { return o.flags[0].enabled }
func (o operationsModel) forceVal() bool         { return o.flags[1].enabled }
func (o operationsModel) initialDeployVal() bool { return o.flags[2].enabled }

func (o *operationsModel) startDeploy() {
	o.running = true
	o.done = false
	o.err = nil
	o.logBuf.Reset()
	o.status = ""
	o.logCh = make(chan string, 256)
	o.viewport.SetContent("")
}

func (o *operationsModel) appendLog(chunk string) {
	o.logBuf.WriteString(chunk)
	o.viewport.SetContent(o.logBuf.String())
	o.viewport.GotoBottom()
}

// startDeploy launches the deployer in a goroutine and returns the first log tea.Cmd.
func startDeploy(cfg *config.Config, envName, repoPath string, dryRun, force, initialDeploy bool, ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			log := logger.NewTUILogger(&logCapture{ch: ch}, true, false)
			d, err := deployer.NewDeployer(cfg, envName, repoPath, dryRun, initialDeploy, force, false, log)
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

func (o operationsModel) view(width int, currentRelease string) string {
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	title := StyleTitle.Render("  Operations")

	rows := []string{"", title, ""}

	// ── DEPLOY SECTION ─────────────────────────────
	rows = append(rows,
		StyleSection.Render("  ▶  Deploy"),
		StyleHint.Render("     Deploy current repository state to the remote server"),
		"",
	)

	for i, f := range o.flags {
		check := StyleMuted.Render("[ ]")
		if f.enabled {
			check = StyleSuccess.Render("[✓]")
		}
		desc := StyleHint.Render("  " + f.desc)
		line := fmt.Sprintf("  %s  %-20s%s", check, f.label, desc)
		if i == o.cursor && !o.running {
			line = StyleSelected.Render(fmt.Sprintf("  %s  %-20s", check, f.label)) + desc
		}
		rows = append(rows, line)
	}

	deployKey := StyleCmd.Render("[D]")
	if o.running {
		deployKey = StyleWarning.Render("[D] running…")
	} else if o.done {
		deployKey = StyleMuted.Render("[D] done — press D again to re-deploy")
	}
	rows = append(rows, "", fmt.Sprintf("  %s Start deploy   ", deployKey))

	// ── ROLLBACK SECTION ───────────────────────────
	rows = append(rows,
		"",
		sep,
		"",
		StyleSection.Render("  ◀  Rollback"),
	)
	if currentRelease != "" {
		rows = append(rows, StyleHint.Render("     Current release: "+currentRelease))
	}
	rows = append(rows,
		StyleHint.Render("     Revert to the release immediately before the current one"),
		"",
		fmt.Sprintf("  %s Rollback to previous release", StyleCmd.Render("[R]")),
		"",
		StyleMuted.Render("     To rollback to a specific version → go to view 2 (Releases)"),
	)

	rows = append(rows, "", sep)

	// ── STATUS MESSAGE ─────────────────────────────
	if o.status != "" {
		rows = append(rows, "", "  "+o.status)
	}

	// ── LOG VIEWPORT ───────────────────────────────
	if o.running || o.done || o.logBuf.Len() > 0 {
		rows = append(rows, "", StyleSection.Render("  Output log:"), "")
		rows = append(rows, o.viewport.View())
	}

	if o.done {
		rows = append(rows, "")
		if o.err != nil {
			rows = append(rows, StyleError.Render("  ✕ Deploy failed: "+o.err.Error()))
		} else {
			rows = append(rows, StyleSuccess.Render("  ✓ Operation completed successfully"))
		}
	}

	// ── HINTS ──────────────────────────────────────
	rows = append(rows, "",
		StyleMuted.Render("  ↑/↓ navigate options   ↵ toggle   D deploy   R rollback"),
	)

	return strings.Join(rows, "\n")
}
