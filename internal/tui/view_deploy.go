package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
)

// logCapture is an io.Writer that sends each write to a channel.
type logCapture struct {
	ch chan string
}

func (lc *logCapture) Write(p []byte) (int, error) {
	lc.ch <- string(p)
	return len(p), nil
}

type msgDeployLogLine struct {
	line string
}

type msgDeployDone struct {
	err error
}

// deployOption is a toggleable deploy flag.
type deployOption struct {
	label   string
	value   *bool
	enabled bool
}

type deployModel struct {
	viewport    viewport.Model
	logLines    []string
	logCh       chan string
	running     bool
	done        bool
	err         error
	cursor      int

	// deploy options
	dryRun        bool
	force         bool
	initialDeploy bool

	options []deployOption
}

func newDeployModel() deployModel {
	m := deployModel{
		logCh: make(chan string, 256),
	}
	m.options = []deployOption{
		{label: "Dry run (no changes)", enabled: false},
		{label: "Force redeploy", enabled: false},
		{label: "Initial deploy", enabled: false},
	}
	return m
}

func (d *deployModel) initViewport(width, height int) {
	d.viewport = viewport.New(width-2, max(height-16, 4))
	d.viewport.Style = StyleBorder
}

func (d *deployModel) toggleOption() {
	if d.cursor >= len(d.options) {
		return
	}
	d.options[d.cursor].enabled = !d.options[d.cursor].enabled
}

func (d *deployModel) moveUp() {
	if d.cursor > 0 {
		d.cursor--
	}
}

func (d *deployModel) moveDown() {
	if d.cursor < len(d.options)-1 {
		d.cursor++
	}
}

func (d *deployModel) appendLog(line string) {
	d.logLines = append(d.logLines, line)
	d.viewport.SetContent(strings.Join(d.logLines, ""))
	d.viewport.GotoBottom()
}

func (d deployModel) dryRunVal() bool        { return d.options[0].enabled }
func (d deployModel) forceVal() bool         { return d.options[1].enabled }
func (d deployModel) initialDeployVal() bool { return d.options[2].enabled }

// startDeploy launches the deploy in a goroutine and returns the first tea.Cmd for log streaming.
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
			err = d.Deploy()
			if err != nil {
				ch <- fmt.Sprintf("[ERROR] %v\n", err)
			}
			close(ch)
		}()
		// Return first log line (or deploy done if no logs)
		return waitForLogLine(ch)()
	}
}

// waitForLogLine returns a tea.Cmd that blocks until the next log line or channel close.
func waitForLogLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgDeployDone{}
		}
		return msgDeployLogLine{line: line}
	}
}

func (d deployModel) view(width int) string {
	title := StyleTitle.Render("  Deploy")
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))

	rows := []string{"", title, "", sep, "", "  Options:"}

	for i, opt := range d.options {
		check := "[ ]"
		if opt.enabled {
			check = StyleSuccess.Render("[✓]")
		}
		line := fmt.Sprintf("  %s %s", check, opt.label)
		if i == d.cursor && !d.running {
			line = StyleSelected.Render(fmt.Sprintf("  %s %s ", check, opt.label))
		}
		rows = append(rows, line)
	}

	rows = append(rows, "")

	if d.running || d.done {
		rows = append(rows, sep, "", "  Deploy log:")
		rows = append(rows, d.viewport.View())
	}

	if d.done {
		rows = append(rows, "")
		if d.err != nil {
			rows = append(rows, StyleError.Render("  Deploy failed: "+d.err.Error()))
		} else {
			rows = append(rows, StyleSuccess.Render("  Deploy completed successfully!"))
		}
	}

	rows = append(rows, "", sep)
	if d.running {
		rows = append(rows, StyleWarning.Render("  Deploy in progress…"))
	} else if !d.done {
		rows = append(rows, StyleMuted.Render("  ↑/↓:navigate  ↵:toggle  d:start deploy"))
	}

	return strings.Join(rows, "\n")
}
