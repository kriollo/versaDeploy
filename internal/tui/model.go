package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
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
	{"1", "Dashboard"},
	{"2", "Releases"},
	{"3", "Files"},
	{"4", "Shared"},
	{"5", "Operations"},
	{"6", "Config"},
	{"7", "Switch Config"},
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

	case tea.KeyMsg:
		return m.handleKey(msg, cmds)
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) handleKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
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

	// View switching — FIX: init browser on ACTUAL model here, not in a value-copy helper
	switch msg.String() {
	case "1":
		m.currentView = viewDashboard
		cmds = append(cmds, m.loadCurrentViewCmds()...)
		return m, tea.Batch(cmds...)
	case "2":
		m.currentView = viewReleases
		cmds = append(cmds, m.loadCurrentViewCmds()...)
		return m, tea.Batch(cmds...)
	case "3":
		m.currentView = viewBrowser
		if env := m.activeEnvCfg(); env != nil {
			m.browser.init(env.RemotePath) // init on real model
			if client := m.activeClient(); client != nil {
				cmds = append(cmds, listDir(client, env.RemotePath))
			}
		}
		return m, tea.Batch(cmds...)
	case "4":
		m.currentView = viewShared
		cmds = append(cmds, m.loadCurrentViewCmds()...)
		return m, tea.Batch(cmds...)
	case "5":
		m.currentView = viewOperations
		return m, tea.Batch(cmds...)
	case "6":
		m.currentView = viewConfig
		m.config.loading = true
		cmds = append(cmds, m.config.load())
		return m, tea.Batch(cmds...)
	case "7":
		m.discoverConfigs()
		return m, tea.Batch(cmds...)
	}

	// Shortcut: d goes to operations view from anywhere
	if key.Matches(msg, Keys.Deploy) && m.currentView != viewOperations {
		m.currentView = viewOperations
		return m, tea.Batch(cmds...)
	}

	// Content-area key handling per view
	switch m.currentView {
	case viewReleases:
		switch {
		case key.Matches(msg, Keys.Up):
			m.releases.moveUp()
		case key.Matches(msg, Keys.Down):
			m.releases.moveDown()
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
				// Navigate into directory
				m.browser.pushDir(dirPath)
				if client != nil {
					cmds = append(cmds, listDir(client, dirPath))
				}
			case filePath != "" && client != nil:
				// Open file viewer
				m.browser.viewing = true
				m.browser.viewPath = filePath
				m.browser.viewLoading = true
				m.statusMsg = "Loading " + filePath + "…"
				cmds = append(cmds, loadFileContent(client, filePath, fileInfo))
			default:
				// ".." virtual entry or no selection — go up
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
			// Only reload if we actually moved up (not just closed viewer)
			if !m.browser.viewing && m.browser.currentPath() != prevPath {
				if client := m.activeClient(); client != nil {
					cmds = append(cmds, listDir(client, m.browser.currentPath()))
				}
			}
		}

	case viewOperations:
		cmds = append(cmds, m.handleOperationsKey(msg)...)
	case viewConfig:
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
					cmds = append(cmds, m.loadCurrentViewCmds()...)
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

	if m.operations.running {
		return cmds
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

func (m appModel) renderTabBar(contentW int) string {
	var parts []string
	for i, v := range viewLabels {
		label := fmt.Sprintf(" %s:%s ", v.key, v.name)
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
		content = m.dashboard.view(contentW)
	case viewReleases:
		content = m.releases.view(contentW)
	case viewBrowser:
		content = m.browser.view(contentW)
	case viewShared:
		content = m.shared.view(contentW)
	case viewOperations:
		content = m.operations.view(contentW, m.releases.current)
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
