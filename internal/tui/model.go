package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
	versassh "github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/version"
)

type viewID int

const (
	viewDashboard viewID = iota
	viewReleases
	viewBrowser
	viewShared
	viewOperations
	viewTerminal
	viewConfig
	viewConfigSelector
)

type connState int

const (
	connIdle connState = iota
	connConnecting
	connConnected
	connError
)

var viewLabels = []struct {
	key  string
	name string
}{
	{"", "Dashboard"},
	{"", "Releases"},
	{"", "Files"},
	{"", "Shared"},
	{"", "Operations"},
	{"", "Terminal"},
	{"", "Config"},
	{"", "Switch Config"},
}

type msgConnected struct {
	envName string
	client  *versassh.Client
}

type msgConnError struct {
	envName string
	err     error
}

type appModel struct {
	cfg      *config.Config
	repoPath string
	width    int
	height   int

	envNames       []string
	activeEnv      int
	sidebarFocused bool

	sshClients map[string]*versassh.Client
	connStates map[string]connState
	connErrors map[string]error

	currentView viewID

	dashboard  dashboardModel
	releases   releasesModel
	browser    browserModel
	shared     sharedModel
	operations operationsModel
	terminal   terminalModel
	config     configModel

	spinner   spinner.Model
	statusMsg string

	// Config selection
	availableConfigs []string
	configSelector   configSelectorModel
}

type configSelectorModel struct {
	cursor int
}

func newAppModel(cfg *config.Config, repoPath string) appModel {
	var names []string
	if cfg != nil {
		names = make([]string, 0, len(cfg.Environments))
		for k := range cfg.Environments {
			names = append(names, k)
		}
		sort.Strings(names)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleConnecting

	return appModel{
		cfg:         cfg,
		repoPath:    repoPath,
		envNames:    names,
		sshClients:  make(map[string]*versassh.Client),
		connStates:  make(map[string]connState),
		connErrors:  make(map[string]error),
		spinner:     sp,
		operations:  newOperationsModel(),
		terminal:    newTerminalModel(),
		config:      newConfigModel(repoPath),
		currentView: viewDashboard,
	}
}

func (m *appModel) discoverConfigs() {
	cwd, _ := os.Getwd()
	m.availableConfigs, _ = config.FindConfigFiles(cwd)

	// If we have multiple configs, or no config loaded, show selector
	if len(m.availableConfigs) > 1 || m.cfg == nil {
		m.currentView = viewConfigSelector
	}
}

func (m appModel) activeEnvName() string {
	if len(m.envNames) == 0 {
		return ""
	}
	return m.envNames[m.activeEnv]
}

func (m appModel) activeEnvCfg() *config.Environment {
	env, err := m.cfg.GetEnvironment(m.activeEnvName())
	if err != nil {
		return nil
	}
	return env
}

func (m appModel) activeClient() *versassh.Client {
	return m.sshClients[m.activeEnvName()]
}

func (m appModel) isConnected() bool {
	return m.connStates[m.activeEnvName()] == connConnected
}

func connectEnvCmd(cfg *config.Config, envName string) tea.Cmd {
	return func() tea.Msg {
		envCfg, err := cfg.GetEnvironment(envName)
		if err != nil {
			return msgConnError{envName: envName, err: err}
		}
		log, _ := logger.NewLogger("", false, false)
		client, err := versassh.NewClient(&envCfg.SSH, log)
		if err != nil {
			return msgConnError{envName: envName, err: err}
		}
		return msgConnected{envName: envName, client: client}
	}
}

func (m appModel) renderConfigSelector(width int) string {
	header := StyleHeader.Width(width).Render("Select Configuration File") + "\n\n"

	var lines []string
	for i, f := range m.availableConfigs {
		label := filepath.Base(f)
		if i == m.configSelector.cursor {
			lines = append(lines, StyleSelected.Render(fmt.Sprintf("> %s", label)))
		} else {
			lines = append(lines, fmt.Sprintf("  %s", label))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("No configuration files found."))
	}

	body := strings.Join(lines, "\n")
	footer := "\n\n" + StyleMuted.Render("[↑/↓: move, Enter: select, Esc: cancel]")

	return header + body + footer
}

func (m appModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if len(m.envNames) > 0 {
		name := m.envNames[0]
		m.connStates[name] = connConnecting
		cmds = append(cmds, connectEnvCmd(m.cfg, name))
	}
	return tea.Batch(cmds...)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := m.height - 3 // header + tabbar + statusbar
		contentW := m.width - sidebarWidth - 2
		if contentW < 10 {
			contentW = 10
		}
		m.operations.initViewport(contentW, contentH)
		m.terminal.initViewport(contentW, contentH)
		m.config.contentH = contentH

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case msgConnected:
		m.sshClients[msg.envName] = msg.client
		m.connStates[msg.envName] = connConnected
		delete(m.connErrors, msg.envName)
		m.statusMsg = "Connected to " + msg.envName
		if msg.envName == m.activeEnvName() {
			// FIX: init browser on the actual model before loading
			if m.currentView == viewBrowser {
				if env := m.activeEnvCfg(); env != nil {
					m.browser.init(env.RemotePath)
				}
			}
			// Check deploy.lock existence to lock/unlock "Initial deploy" flag
			if env := m.activeEnvCfg(); env != nil {
				lockPath := filepath.ToSlash(filepath.Join(env.RemotePath, "deploy.lock"))
				if exists, err := msg.client.FileExists(lockPath); err == nil {
					m.operations.deployLockExists = exists
					if exists {
						m.operations.flags[2].enabled = false
					}
				}
			}
			cmds = append(cmds, m.loadCurrentViewCmds()...)
		}

	case msgConnError:
		m.connStates[msg.envName] = connError
		m.connErrors[msg.envName] = msg.err
		m.statusMsg = "Connection failed: " + msg.err.Error()

	case msgDashboardData:
		m.dashboard.applyData(msg)

	case msgReleasesLoaded:
		m.releases.applyLoaded(msg)

	case msgRollbackDone:
		if msg.err != nil {
			m.releases.status = StyleError.Render("Rollback failed: " + msg.err.Error())
			m.operations.status = StyleError.Render("Rollback failed: " + msg.err.Error())
		} else {
			m.releases.status = StyleSuccess.Render("Rollback successful!")
			m.operations.status = StyleSuccess.Render("Rollback successful!")
			if client := m.activeClient(); client != nil {
				if env := m.activeEnvCfg(); env != nil {
					cmds = append(cmds, loadReleases(client, env.RemotePath))
					cmds = append(cmds, loadDashboard(client, env.RemotePath))
				}
			}
		}

	case msgDirListed:
		m.browser.applyListed(msg)

	case msgFileContent:
		contentW := m.width - sidebarWidth - 6
		contentH := m.height - 3
		m.browser.applyFileContent(msg, contentW, contentH)
		m.statusMsg = "Viewing: " + msg.path

	case msgXferLine:
		m.browser.appendXferLine(msg.line)
		cmds = append(cmds, waitForXferLine(m.browser.xfer.logCh))

	case msgXferStreamEnd:
		// log channel closed; msgXferDone arrives separately via doneCh

	case msgDeleteDone:
		if msg.err != nil {
			m.browser.statusMsg = StyleError.Render("\U000F0159 Delete failed: " + msg.err.Error()) // mdi-close-circle
		} else {
			m.browser.statusMsg = StyleSuccess.Render("\U000F012C Deleted: " + msg.path) // mdi-check-circle
			if client := m.activeClient(); client != nil {
				cmds = append(cmds, listDir(client, m.browser.currentPath()))
			}
		}

	case msgFileSaved:
		m.browser.editSaving = false
		if msg.err != nil {
			m.browser.statusMsg = StyleError.Render("\U000F0159 Save failed: " + msg.err.Error())
		} else {
			m.browser.editing = false
			m.browser.statusMsg = StyleSuccess.Render("\U000F012C Saved: " + msg.path)
			// Reload file content to reflect saved changes
			if client := m.activeClient(); client != nil {
				m.browser.viewLoading = true
				cmds = append(cmds, loadFileContent(client, msg.path, m.browser.viewInfo))
			}
		}

	case msgXferDone:
		m.browser.xfer.running = false
		m.browser.xfer.done = true
		m.browser.xfer.err = msg.err
		if msg.err == nil && m.browser.xfer.dir == xferUpload {
			if client := m.activeClient(); client != nil {
				cmds = append(cmds, listDir(client, m.browser.currentPath()))
			}
		}

	case msgSharedData:
		m.shared.applyData(msg)

	case msgDeployLogLine:
		m.operations.appendLog(msg.line)
		cmds = append(cmds, waitForLogLine(m.operations.logCh))

	case msgDeployDone:
		m.operations.running = false
		m.operations.done = true
		// Refresh dashboard after deploy
		if client := m.activeClient(); client != nil {
			if env := m.activeEnvCfg(); env != nil {
				cmds = append(cmds, loadDashboard(client, env.RemotePath))
				cmds = append(cmds, loadReleases(client, env.RemotePath))
			}
		}

	case msgConfigLoaded:
		m.config.applyLoaded(msg)

	case msgConfigSaved:
		m.config.applySaved(msg)

	case msgTerminalOutput:
		m.terminal.appendOutput(msg.line)
		cmds = append(cmds, waitForTermLine(m.terminal.logCh))

	case msgTerminalDone:
		m.terminal.running = false
		if msg.err != nil {
			m.terminal.appendOutput(fmt.Sprintf("\n[ERROR] %v\n", msg.err))
		}
		m.terminal.appendOutput("\n")
		m.terminal.input.Focus()

	case msgCdResolved:
		if msg.err != nil {
			m.terminal.appendOutput(fmt.Sprintf("cd: %v\n\n", msg.err))
		} else {
			m.terminal.cwd = msg.newCwd
			m.terminal.appendOutput(fmt.Sprintf("%s\n\n", msg.newCwd))
		}

	case msgTabComplete:
		if msg.err == nil && len(msg.matches) > 0 {
			m.terminal.completions = msg.matches
			m.terminal.completionIdx = 0
			m.terminal.completionOn = true
			m.terminal.completionPfx = msg.prefix
			m.terminal.completionTok = msg.token
			// Apply first match
			m.terminal.input.SetValue(msg.prefix + msg.matches[0])
			m.terminal.input.CursorEnd()
		}

	case tea.MouseMsg:
		if m.currentView == viewTerminal {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.terminal.vp.LineUp(3)
			case tea.MouseButtonWheelDown:
				m.terminal.vp.LineDown(3)
			}
		} else if m.currentView == viewOperations {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.operations.scrollUp()
			case tea.MouseButtonWheelDown:
				m.operations.scrollDown()
			}
		}

	case tea.KeyMsg:
		return m.handleKey(msg, cmds)
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) handleKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	// While editing a remote file in the browser, route all key input to the textarea.
	if m.currentView == viewBrowser && m.browser.editing {
		switch msg.String() {
		case "ctrl+s":
			if !m.browser.editSaving {
				m.browser.editSaving = true
				if client := m.activeClient(); client != nil {
					content := []byte(m.browser.editTextarea.Value())
					cmds = append(cmds, saveRemoteFileCmd(client, m.browser.editPath, content))
				}
			}
		case "esc":
			m.browser.editing = false
		default:
			var taCmd tea.Cmd
			m.browser.editTextarea, taCmd = m.browser.editTextarea.Update(msg)
			if taCmd != nil {
				cmds = append(cmds, taCmd)
			}
		}
		return m, tea.Batch(cmds...)
	}

	// While editing config, route all key input to the editor and block global shortcuts.
	if m.currentView == viewConfig && m.config.isEditing {
		handled, _ := (&m.config).handleKey(msg)
		cmds = append(cmds, handled...)
		return m, tea.Batch(cmds...)
	}

	// While in terminal view, route all keys to the terminal and block global shortcuts.
	// Only allow Ctrl+C to quit and number keys to switch views when not typing.
	if m.currentView == viewTerminal {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "esc" && !m.terminal.running {
			// Esc exits terminal view back to dashboard
			cmds = append(cmds, m.switchToView(viewDashboard)...)
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.handleTerminalKey(msg)...)
		return m, tea.Batch(cmds...)
	}

	if key.Matches(msg, Keys.Quit) {
		return m, tea.Quit
	}

	if key.Matches(msg, Keys.Tab) {
		if m.currentView == viewConfig && m.config.isEditing {
			// En modo edición de config, no procesamos Tab
		} else {
			m.sidebarFocused = !m.sidebarFocused
			return m, tea.Batch(cmds...)
		}
	}

	if key.Matches(msg, Keys.Connect) {
		envName := m.activeEnvName()
		m.connStates[envName] = connConnecting
		if c := m.sshClients[envName]; c != nil {
			c.Close()
			delete(m.sshClients, envName)
		}
		cmds = append(cmds, connectEnvCmd(m.cfg, envName))
		return m, tea.Batch(cmds...)
	}

	// Sidebar navigation
	if m.sidebarFocused {
		switch {
		case key.Matches(msg, Keys.Up):
			if m.activeEnv > 0 {
				m.activeEnv--
				m = m.resetViewState()
				cmds = append(cmds, m.autoConnectCmds()...)
			}
		case key.Matches(msg, Keys.Down):
			if m.activeEnv < len(m.envNames)-1 {
				m.activeEnv++
				m = m.resetViewState()
				cmds = append(cmds, m.autoConnectCmds()...)
			}
		case key.Matches(msg, Keys.Enter):
			m.sidebarFocused = false
		}
		return m, tea.Batch(cmds...)
	}

	// Left/Right arrow: cycle through views (skip viewConfigSelector=6)
	// Allow from config view when not editing.
	configBlocked := m.currentView == viewConfig && m.config.isEditing
	if key.Matches(msg, Keys.Left) && !m.sidebarFocused && m.currentView != viewConfigSelector && !configBlocked {
		next := int(m.currentView) - 1
		if next < 0 {
			next = int(viewConfig)
		}
		cmds = append(cmds, m.switchToView(viewID(next))...)
		return m, tea.Batch(cmds...)
	}
	if key.Matches(msg, Keys.Right) && !m.sidebarFocused && m.currentView != viewConfigSelector && !configBlocked {
		next := int(m.currentView) + 1
		if next > int(viewConfig) {
			next = 0
		}
		cmds = append(cmds, m.switchToView(viewID(next))...)
		return m, tea.Batch(cmds...)
	}



	// Shortcut: d goes to operations view from anywhere
	if key.Matches(msg, Keys.Deploy) && m.currentView != viewOperations && m.currentView != viewBrowser {
		m.currentView = viewOperations
		return m, tea.Batch(cmds...)
	}

	// F5 refresh: reload data for current view
	if key.Matches(msg, Keys.Refresh) {
		cmds = append(cmds, m.loadCurrentViewCmds()...)
		m.statusMsg = "Refreshing…"
		return m, tea.Batch(cmds...)
	}

	// Content-area key handling per view
	switch m.currentView {
	case viewDashboard:
		// Dashboard handles r (Refresh already handled above via F5)

	case viewReleases:
		switch {
		case key.Matches(msg, Keys.Up):
			m.releases.moveUp()
		case key.Matches(msg, Keys.Down):
			m.releases.moveDown()
		case key.Matches(msg, Keys.Enter):
			rel := m.releases.selectedRelease()
			if rel != "" {
				if env := m.activeEnvCfg(); env != nil {
					relPath := filepath.ToSlash(filepath.Join(env.RemotePath, "releases", rel))
					m.currentView = viewBrowser
					m.browser.init(relPath)
					if client := m.activeClient(); client != nil {
						cmds = append(cmds, listDir(client, relPath))
					}
				}
			}
		case key.Matches(msg, Keys.Rollback):
			rel := m.releases.selectedRelease()
			if rel != "" && rel != m.releases.current {
				if client := m.activeClient(); client != nil {
					if env := m.activeEnvCfg(); env != nil {
						m.releases.status = StyleWarning.Render("Rolling back to " + rel + "…")
						cmds = append(cmds, doRollback(client, env.RemotePath, rel))
					}
				}
			}
		}

	case viewBrowser:
		// ── Transfer panel active ──────────────────────────────────────────
		if m.browser.xfer.dir != xferNone {
			x := &m.browser.xfer

			// Log view (running or done): Esc always closes, Enter closes when done
			if x.running || x.done {
				switch msg.String() {
				case "esc":
					m.browser.cancelXfer()
				case "enter":
					if x.done {
						m.browser.cancelXfer()
					}
				}
				break
			}

			// Both upload and download use the local browser for navigation
			switch msg.String() {
			case "esc":
				m.browser.cancelXfer()
			case "up":
				x.local.moveUp()
			case "down":
				x.local.moveDown()
			case "enter":
				// Navigate into directories; for upload also toggles file selection
				x.local.enterSelected()
			case " ":
				// Toggle file selection (upload only; ignored for download)
				if x.dir == xferUpload {
					x.local.spaceSelect()
				}
			case "backspace":
				x.local.popDir()
			case "t":
				env := m.activeEnvCfg()
				if env == nil {
					break
				}
				if x.dir == xferUpload {
					sel := x.local.selectedFiles()
					if len(sel) > 0 {
						dst := fmt.Sprintf("%s@%s:%s/", x.sshUser, x.sshHost, x.remotePath)
						x.running = true
						cmds = append(cmds,
							startRsyncCmd(x.sshCfg, sel, dst, x.logCh, x.doneCh),
							waitForXferDone(x.doneCh),
						)
					}
				} else {
					// Download: save remote file into the currently browsed local dir
					src := fmt.Sprintf("%s@%s:%s", x.sshUser, x.sshHost, x.remotePath)
					dst := x.local.currentPath() + "/"
					x.running = true
					cmds = append(cmds,
						startRsyncCmd(x.sshCfg, []string{src}, dst, x.logCh, x.doneCh),
						waitForXferDone(x.doneCh),
					)
				}
			}
			break
		}

		// ── Normal remote browser ──────────────────────────────────────────
		// Esc closes the file viewer
		if msg.String() == "esc" && m.browser.viewing {
			m.browser.viewing = false
			break
		}
		switch {
		case key.Matches(msg, Keys.Up):
			m.browser.moveUp()
		case key.Matches(msg, Keys.Down):
			m.browser.moveDown()
		case key.Matches(msg, Keys.Enter):
			client := m.activeClient()
			dirPath, filePath, fileInfo := m.browser.enterSelected()
			switch {
			case dirPath != "":
				m.browser.pushDir(dirPath)
				if client != nil {
					cmds = append(cmds, listDir(client, dirPath))
				}
			case filePath != "" && client != nil:
				m.browser.viewing = true
				m.browser.viewPath = filePath
				m.browser.viewLoading = true
				m.statusMsg = "Loading " + filePath + "…"
				cmds = append(cmds, loadFileContent(client, filePath, fileInfo))
			default:
				if m.browser.hasParent() && !m.browser.viewing {
					m.browser.popDir()
					if client != nil {
						cmds = append(cmds, listDir(client, m.browser.currentPath()))
					}
				}
			}
		case key.Matches(msg, Keys.Back):
			prevPath := m.browser.currentPath()
			m.browser.popDir()
			if !m.browser.viewing && m.browser.currentPath() != prevPath {
				if client := m.activeClient(); client != nil {
					cmds = append(cmds, listDir(client, m.browser.currentPath()))
				}
			}
		default:
			env := m.activeEnvCfg()
			if env == nil {
				break
			}
			switch msg.String() {
			case "d":
				var remotePath string
				if m.browser.viewing {
					remotePath = m.browser.viewPath
				} else {
					remotePath = m.browser.selectedRemoteFile()
				}
				if remotePath != "" {
					m.browser.initDownload(remotePath, env.SSH.User, env.SSH.Host, &env.SSH)
				}
			case "u":
				if !m.browser.viewing {
					m.browser.initUpload(m.browser.currentPath(), env.SSH.User, env.SSH.Host, &env.SSH)
				}
			case "delete":
				remotePath := m.browser.selectedRemoteFile()
				if remotePath != "" {
					if client := m.activeClient(); client != nil {
						m.browser.statusMsg = StyleMuted.Render("Deleting…")
						cmds = append(cmds, deleteFileCmd(client, remotePath))
					}
				}
			case "e":
				// Enter edit mode for text files with valid UTF-8 content
				if m.browser.viewing {
					name := filepath.Base(m.browser.viewPath)
					const maxRead = 512 * 1024
					if detectKind(name) == kindText && utf8.Valid(m.browser.viewContent) && int64(len(m.browser.viewContent)) < maxRead {
						contentW := m.width - sidebarWidth - 6
						contentH := m.height - 3
						taH := contentH - 8
						if taH < 4 {
							taH = 4
						}
						ta := textarea.New()
						ta.SetWidth(contentW - 4)
						ta.SetHeight(taH)
						ta.SetValue(string(m.browser.viewContent))
						ta.Focus()
						m.browser.editTextarea = ta
						m.browser.editPath = m.browser.viewPath
						m.browser.editSaving = false
						m.browser.editing = true
					}
				}
			}
		}

	case viewShared:
		switch {
		case key.Matches(msg, Keys.Up):
			m.shared.moveUp()
		case key.Matches(msg, Keys.Down):
			m.shared.moveDown()
		case key.Matches(msg, Keys.Enter):
			if entry := m.shared.selectedEntry(); entry != nil && entry.isDir {
				if env := m.activeEnvCfg(); env != nil {
					dirPath := filepath.ToSlash(filepath.Join(env.RemotePath, "shared", entry.name))
					m.currentView = viewBrowser
					m.browser.init(dirPath)
					if client := m.activeClient(); client != nil {
						cmds = append(cmds, listDir(client, dirPath))
					}
				}
			}
		}

	case viewOperations:
		cmds = append(cmds, m.handleOperationsKey(msg)...)

	case viewTerminal:
		cmds = append(cmds, m.handleTerminalKey(msg)...)

	case viewConfig:
		// Esc in read-only mode navigates back to dashboard
		if msg.String() == "esc" && !m.config.isEditing {
			m.currentView = viewDashboard
			cmds = append(cmds, m.loadCurrentViewCmds()...)
			return m, tea.Batch(cmds...)
		}
		handled, stop := (&m.config).handleKey(msg)
		cmds = append(cmds, handled...)
		if stop {
			return m, tea.Batch(cmds...)
		}
	case viewConfigSelector:
		switch {
		case key.Matches(msg, Keys.Up):
			if m.configSelector.cursor > 0 {
				m.configSelector.cursor--
			}
		case key.Matches(msg, Keys.Down):
			if m.configSelector.cursor < len(m.availableConfigs)-1 {
				m.configSelector.cursor++
			}
		case key.Matches(msg, Keys.Enter):
			if len(m.availableConfigs) > 0 {
				path := m.availableConfigs[m.configSelector.cursor]
				cfg, err := config.Load(path)
				if err == nil {
					m.cfg = cfg
					m.config.configPath = path
					m.config.loaded = false
					m.activeEnv = 0
					m.envNames = make([]string, 0, len(cfg.Environments))
					for k := range cfg.Environments {
						m.envNames = append(m.envNames, k)
					}
					sort.Strings(m.envNames)
					m.currentView = viewDashboard
					m.statusMsg = "Switched to " + filepath.Base(path)
					// Reset SSH state for the new config environments
					m.sshClients = make(map[string]*versassh.Client)
					m.connStates = make(map[string]connState)
					m.connErrors = make(map[string]error)
					cmds = append(cmds, m.autoConnectCmds()...)
				} else {
					m.statusMsg = "Error loading config: " + err.Error()
				}
			}
		case msg.String() == "esc":
			m.currentView = viewDashboard
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *appModel) handleOperationsKey(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// While editing log file path, route keys to the text input
	if m.operations.editingLogFile {
		switch msg.String() {
		case "enter":
			m.operations.confirmLogFile()
			return cmds
		case "esc":
			m.operations.cancelLogFileEdit()
			return cmds
		default:
			var tiCmd tea.Cmd
			m.operations.logFileInput, tiCmd = m.operations.logFileInput.Update(msg)
			if tiCmd != nil {
				cmds = append(cmds, tiCmd)
			}
			return cmds
		}
	}

	if m.operations.running || m.operations.done {
		// Route scroll keys to viewport
		switch msg.String() {
		case "up":
			m.operations.scrollUp()
			return cmds
		case "down":
			m.operations.scrollDown()
			return cmds
		case "pgup":
			m.operations.scrollPageUp()
			return cmds
		case "pgdown":
			m.operations.scrollPageDown()
			return cmds
		}
	}

	if m.operations.running {
		// Allow Esc to close the log even while running
		if msg.String() == "esc" {
			m.operations.clearLog()
		}
		return cmds
	}

	// Esc clears the log output and returns to the idle control panel
	if msg.String() == "esc" && m.operations.done {
		m.operations.clearLog()
		return cmds
	}

	// When cursor is on the log file path row (index == len(flags)), Enter/Space opens edit mode
	logFileIdx := len(m.operations.flags)
	if m.operations.cursor == logFileIdx {
		switch msg.String() {
		case "enter", " ":
			m.operations.enterLogFileEdit()
			return cmds
		}
	}

	switch {
	case key.Matches(msg, Keys.Up):
		m.operations.moveUp()
	case key.Matches(msg, Keys.Down):
		m.operations.moveDown()
	case key.Matches(msg, Keys.Enter):
		m.operations.toggleOption()
	case key.Matches(msg, Keys.Deploy):
		if !m.operations.deployRunning() {
			envName := m.activeEnvName()
			if m.activeEnvCfg() != nil {
				m.operations.startDeploy()
				cmds = append(cmds, startDeploy(
					m.cfg, envName, m.repoPath,
					m.operations.dryRunVal(),
					m.operations.forceVal(),
					m.operations.initialDeployVal(),
					m.operations.skipDirtyCheckVal(),
					m.operations.debugModeVal(),
					m.operations.logFilePath,
					m.operations.logCh,
				))
			}
		}
	case key.Matches(msg, Keys.Rollback):
		if !m.operations.deployRunning() {
			envName := m.activeEnvName()
			if env := m.activeEnvCfg(); env != nil {
				if client := m.sshClients[envName]; client != nil {
					m.operations.status = StyleWarning.Render("Rolling back to previous release…")
					cmds = append(cmds, doRollbackToPrevious(client, env.RemotePath))
				}
			}
		}
	}

	// Quick actions
	if !m.operations.running {
		switch msg.String() {
		case "s":
			if m.cfg != nil {
				m.operations.startDeploy()
				m.operations.logCh = make(chan string, 256)
				cmds = append(cmds, doSSHTest(m.cfg, m.activeEnvName(), m.operations.logCh))
			}
		case "u":
			m.operations.startDeploy()
			m.operations.logCh = make(chan string, 256)
			cmds = append(cmds, doSelfUpdate(m.operations.logCh))
		case "t":
			if client := m.activeClient(); client != nil {
				if env := m.activeEnvCfg(); env != nil {
					m.operations.startDeploy()
					m.operations.logCh = make(chan string, 256)
					cmds = append(cmds, doStatus(client, env.RemotePath, m.operations.logCh))
				}
			}
		case "h":
			// Re-execute post_deploy hooks on the active release
			envName := m.activeEnvName()
			if m.cfg != nil && envName != "" {
				m.operations.startDeploy()
				m.operations.logCh = make(chan string, 256)
				cmds = append(cmds, doRunHooks(m.cfg, envName, m.repoPath, m.operations.logCh))
			}
		case "l":
			// Re-execute services_reload commands
			envName := m.activeEnvName()
			if m.cfg != nil && envName != "" {
				m.operations.startDeploy()
				m.operations.logCh = make(chan string, 256)
				cmds = append(cmds, doServicesReload(m.cfg, envName, m.repoPath, m.operations.logCh))
			}
		}
	}

	return cmds
}

func (m *appModel) handleTerminalKey(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	if m.terminal.running {
		// While a command is running, allow scrolling and Esc
		switch msg.String() {
		case "up":
			m.terminal.vp.LineUp(3)
		case "down":
			m.terminal.vp.LineDown(3)
		case "pgup":
			m.terminal.vp.ViewUp()
		case "pgdown":
			m.terminal.vp.ViewDown()
		}
		return cmds
	}

	switch msg.String() {
	case "enter":
		m.terminal.resetCompletion()
		cmd := strings.TrimSpace(m.terminal.input.Value())
		if cmd != "" && m.isConnected() {
			if client := m.activeClient(); client != nil {
				m.terminal.input.SetValue("")
				// Handle cd commands locally to update cwd
				if strings.HasPrefix(cmd, "cd ") {
					target := strings.TrimSpace(strings.TrimPrefix(cmd, "cd "))
					if target != "" {
						// Resolve the path on the server and update cwd
						resolveCmd := fmt.Sprintf("cd %s && cd %s && pwd", m.terminal.cwd, target)
						m.terminal.logBuf.WriteString(fmt.Sprintf("%s $ %s\n", m.terminal.cwd, cmd))
						m.terminal.history = append(m.terminal.history, cmd)
						m.terminal.histIdx = len(m.terminal.history)
						cmds = append(cmds, resolveCd(client, resolveCmd, m.terminal.cwd))
					}
				} else {
					cmds = append(cmds, m.terminal.executeCommand(client, cmd))
				}
			}
		}
	case "tab":
		if m.isConnected() {
			if client := m.activeClient(); client != nil {
				if m.terminal.completionOn && len(m.terminal.completions) > 1 {
					// Cycle to next completion
					m.terminal.completionIdx = (m.terminal.completionIdx + 1) % len(m.terminal.completions)
					match := m.terminal.completions[m.terminal.completionIdx]
					m.terminal.input.SetValue(m.terminal.completionPfx + match)
					m.terminal.input.CursorEnd()
				} else {
					// Start new completion
					val := m.terminal.input.Value()
					// Find the last token (space-separated) to complete
					prefix := ""
					token := val
					if idx := strings.LastIndex(val, " "); idx >= 0 {
						prefix = val[:idx+1]
						token = val[idx+1:]
					}
					m.terminal.resetCompletion()
					cmds = append(cmds, tabComplete(client, m.terminal.cwd, prefix, token))
				}
			}
		}
	case "up":
		m.terminal.resetCompletion()
		m.terminal.historyUp()
	case "down":
		m.terminal.resetCompletion()
		m.terminal.historyDown()
	case "ctrl+l":
		m.terminal.logBuf = &strings.Builder{}
		m.terminal.vp.SetContent("")
	case "pgup":
		m.terminal.vp.ViewUp()
	case "pgdown":
		m.terminal.vp.ViewDown()
	default:
		m.terminal.resetCompletion()
		var tiCmd tea.Cmd
		m.terminal.input, tiCmd = m.terminal.input.Update(msg)
		if tiCmd != nil {
			cmds = append(cmds, tiCmd)
		}
	}

	return cmds
}

func (m appModel) resetViewState() appModel {
	m.dashboard = dashboardModel{}
	m.releases = releasesModel{}
	m.browser = browserModel{}
	m.shared = sharedModel{}
	return m
}

func (m appModel) autoConnectCmds() []tea.Cmd {
	envName := m.activeEnvName()
	if m.connStates[envName] == connIdle {
		m.connStates[envName] = connConnecting
		return []tea.Cmd{connectEnvCmd(m.cfg, envName)}
	}
	if m.connStates[envName] == connConnected {
		return m.loadCurrentViewCmds()
	}
	return nil
}

// loadCurrentViewCmds returns the tea.Cmds to load data for the current view.
// Does NOT call browser.init — that must be done on the real model before calling this.
func (m appModel) loadCurrentViewCmds() []tea.Cmd {
	client := m.activeClient()
	if client == nil {
		return nil
	}
	env := m.activeEnvCfg()
	if env == nil {
		return nil
	}

	switch m.currentView {
	case viewDashboard:
		return []tea.Cmd{loadDashboard(client, env.RemotePath)}
	case viewReleases:
		return []tea.Cmd{loadReleases(client, env.RemotePath)}
	case viewBrowser:
		// init must have been called on the real model before this
		return []tea.Cmd{listDir(client, m.browser.currentPath())}
	case viewShared:
		return []tea.Cmd{loadShared(client, env.RemotePath)}
	}
	return nil
}

// switchToView switches to the given view and triggers appropriate data loading.
func (m *appModel) switchToView(v viewID) []tea.Cmd {
	m.currentView = v
	if v == viewBrowser {
		if env := m.activeEnvCfg(); env != nil {
			m.browser.init(env.RemotePath)
		}
	}
	if v == viewTerminal {
		if env := m.activeEnvCfg(); env != nil && m.terminal.cwd == "" {
			m.terminal.cwd = env.RemotePath
		}
		m.terminal.input.Focus()
	}
	if v == viewConfig {
		m.config.loading = true
		return []tea.Cmd{m.config.load()}
	}
	return m.loadCurrentViewCmds()
}

func (m appModel) renderTabBar(contentW int) string {
	var parts []string
	for i, v := range viewLabels {
		label := fmt.Sprintf(" %s ", v.name)
		if viewID(i) == m.currentView {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	bar := strings.Join(parts, StyleMuted.Render("│"))
	connHint := ""
	if !m.isConnected() {
		connHint = StyleWarning.Render(" [c:connect] ")
	}
	return StyleSurface.Width(contentW).Render(bar + connHint)
}

func (m appModel) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	envName := m.activeEnvName()
	connected := m.isConnected()

	headerStr := renderHeader(m.width, version.Version, envName, connected)
	statusbarStr := renderStatusbar(m.width, Keys.ShortHelp(), m.statusMsg)

	contentH := m.height - 3 // header + tabbar + statusbar
	sidebar := renderSidebar(contentH+1, m.envNames, m.activeEnv, m.sidebarFocused, m.connStates)

	contentW := m.width - sidebarWidth - 2
	if contentW < 10 {
		contentW = 10
	}

	tabBar := m.renderTabBar(contentW)

	var content string
	switch m.currentView {
	case viewDashboard:
		content = m.dashboard.view(contentW, contentH)
	case viewReleases:
		content = m.releases.view(contentW, contentH)
	case viewBrowser:
		content = m.browser.view(contentW, contentH)
	case viewShared:
		content = m.shared.view(contentW, contentH)
	case viewOperations:
		content = m.operations.view(contentW, contentH, m.releases.current)
	case viewTerminal:
		content = m.terminal.view(contentW, contentH)
	case viewConfig:
		content = m.config.view(contentW)
	case viewConfigSelector:
		content = m.renderConfigSelector(contentW)
	}

	contentPane := lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		StyleContent.Width(contentW).Height(contentH).Render(content),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, contentPane)

	return headerStr + "\n" + body + "\n" + statusbarStr
}
