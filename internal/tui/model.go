package tui

import (
	"sort"

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
	viewDeploy
)

type connState int

const (
	connIdle connState = iota
	connConnecting
	connConnected
	connError
)

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

	dashboard dashboardModel
	releases  releasesModel
	browser   browserModel
	shared    sharedModel
	deploy    deployModel

	spinner   spinner.Model
	statusMsg string
}

func newAppModel(cfg *config.Config, repoPath string) appModel {
	names := make([]string, 0, len(cfg.Environments))
	for k := range cfg.Environments {
		names = append(names, k)
	}
	sort.Strings(names)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleConnecting

	return appModel{
		cfg:        cfg,
		repoPath:   repoPath,
		envNames:   names,
		sshClients: make(map[string]*versassh.Client),
		connStates: make(map[string]connState),
		connErrors: make(map[string]error),
		spinner:    sp,
		deploy:     newDeployModel(),
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

// connectEnvCmd starts an async SSH connection for the given environment.
func connectEnvCmd(cfg *config.Config, envName string) tea.Cmd {
	return func() tea.Msg {
		envCfg, err := cfg.GetEnvironment(envName)
		if err != nil {
			return msgConnError{envName: envName, err: err}
		}
		// Use a no-op logger for the SSH connection during TUI
		log, _ := logger.NewLogger("", false, false)
		client, err := versassh.NewClient(&envCfg.SSH, log)
		if err != nil {
			return msgConnError{envName: envName, err: err}
		}
		return msgConnected{envName: envName, client: client}
	}
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
		contentH := m.height - 2
		contentW := m.width - sidebarWidth - 2
		if contentW < 10 {
			contentW = 10
		}
		m.deploy.initViewport(contentW, contentH)

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
			cmds = append(cmds, m.loadCurrentView()...)
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
		} else {
			m.releases.status = StyleSuccess.Render("Rollback successful!")
			if client := m.activeClient(); client != nil {
				if env := m.activeEnvCfg(); env != nil {
					cmds = append(cmds, loadReleases(client, env.RemotePath))
				}
			}
		}

	case msgDirListed:
		m.browser.applyListed(msg)

	case msgSharedData:
		m.shared.applyData(msg)

	case msgDeployLogLine:
		m.deploy.appendLog(msg.line)
		cmds = append(cmds, waitForLogLine(m.deploy.logCh))

	case msgDeployDone:
		m.deploy.running = false
		m.deploy.done = true

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
		m.sidebarFocused = !m.sidebarFocused
		return m, tea.Batch(cmds...)
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
				m = m.onEnvSwitch()
				cmds = append(cmds, m.autoConnectCmd()...)
			}
		case key.Matches(msg, Keys.Down):
			if m.activeEnv < len(m.envNames)-1 {
				m.activeEnv++
				m = m.onEnvSwitch()
				cmds = append(cmds, m.autoConnectCmd()...)
			}
		case key.Matches(msg, Keys.Enter):
			m.sidebarFocused = false
		}
		return m, tea.Batch(cmds...)
	}

	// View switching
	switch {
	case key.Matches(msg, Keys.View1):
		m.currentView = viewDashboard
		cmds = append(cmds, m.loadCurrentView()...)
	case key.Matches(msg, Keys.View2):
		m.currentView = viewReleases
		cmds = append(cmds, m.loadCurrentView()...)
	case key.Matches(msg, Keys.View3):
		m.currentView = viewBrowser
		cmds = append(cmds, m.loadCurrentView()...)
	case key.Matches(msg, Keys.View4):
		m.currentView = viewShared
		cmds = append(cmds, m.loadCurrentView()...)
	case key.Matches(msg, Keys.View5):
		m.currentView = viewDeploy
	}

	// View-specific keys (only when d is pressed outside deploy view for quick access)
	if key.Matches(msg, Keys.Deploy) && m.currentView != viewDeploy {
		m.currentView = viewDeploy
		return m, tea.Batch(cmds...)
	}

	// Content area navigation
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
			if newPath := m.browser.enterDir(); newPath != "" {
				m.browser.pushDir(newPath)
				if client := m.activeClient(); client != nil {
					cmds = append(cmds, listDir(client, newPath))
				}
			}
		case key.Matches(msg, Keys.Back):
			m.browser.popDir()
			if client := m.activeClient(); client != nil {
				cmds = append(cmds, listDir(client, m.browser.currentPath()))
			}
		}

	case viewDeploy:
		if !m.deploy.running {
			switch {
			case key.Matches(msg, Keys.Up):
				m.deploy.moveUp()
			case key.Matches(msg, Keys.Down):
				m.deploy.moveDown()
			case key.Matches(msg, Keys.Enter):
				m.deploy.toggleOption()
			case key.Matches(msg, Keys.Deploy):
				if !m.deploy.done {
					envName := m.activeEnvName()
					if m.activeEnvCfg() != nil {
						m.deploy.running = true
						m.deploy.done = false
						m.deploy.logLines = nil
						m.deploy.logCh = make(chan string, 256)
						cmds = append(cmds, startDeploy(
							m.cfg, envName, m.repoPath,
							m.deploy.dryRunVal(),
							m.deploy.forceVal(),
							m.deploy.initialDeployVal(),
							m.deploy.logCh,
						))
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m appModel) onEnvSwitch() appModel {
	m.dashboard = dashboardModel{}
	m.releases = releasesModel{}
	m.browser = browserModel{}
	m.shared = sharedModel{}
	return m
}

func (m appModel) autoConnectCmd() []tea.Cmd {
	envName := m.activeEnvName()
	if m.connStates[envName] == connIdle {
		m.connStates[envName] = connConnecting
		return []tea.Cmd{connectEnvCmd(m.cfg, envName)}
	}
	return nil
}

func (m appModel) loadCurrentView() []tea.Cmd {
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
		m.browser.init(env.RemotePath)
		return []tea.Cmd{listDir(client, env.RemotePath)}
	case viewShared:
		return []tea.Cmd{loadShared(client, env.RemotePath, env.SharedPaths)}
	}
	return nil
}

func (m appModel) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	envName := m.activeEnvName()
	connected := m.isConnected()

	headerStr := renderHeader(m.width, version.Version, envName, connected)
	statusbarStr := renderStatusbar(m.width, Keys.ShortHelp(), m.statusMsg)

	contentH := m.height - 2
	sidebar := renderSidebar(contentH, m.envNames, m.activeEnv, m.sidebarFocused, m.connStates)

	contentW := m.width - sidebarWidth - 2
	if contentW < 10 {
		contentW = 10
	}

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
	case viewDeploy:
		content = m.deploy.view(contentW)
	}

	contentPane := StyleContent.Width(contentW).Height(contentH).Render(content)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, contentPane)

	return headerStr + "\n" + body + "\n" + statusbarStr
}
