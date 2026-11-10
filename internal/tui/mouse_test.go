package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/user/versaDeploy/internal/config"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

// Clicking where a tab is drawn switches to it; clicking an environment selects it.
func TestMouseClicksTabsAndEnvironments(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.Environment{"production": {}, "staging": {}}}
	m := newAppModel(cfg, t.TempDir())
	m.splash = false
	m.width, m.height = 170, 40

	for i, v := range viewLabels {
		if viewID(i) == viewConfigSelector {
			continue
		}
		line := ansi.Strip(strings.Split(m.View(), "\n")[tabBarRow])
		at := strings.Index(line, " "+v.name+" ")
		if at < 0 {
			t.Fatalf("tab %q not drawn on row %d: %q", v.name, tabBarRow, line)
		}
		next, _ := m.Update(click(lipgloss.Width(line[:at])+2, tabBarRow))
		if got := next.(appModel).currentView; got != viewID(i) {
			t.Fatalf("click on %q opened view %v", v.name, got)
		}
		m = next.(appModel)
	}

	m.currentView = viewDashboard
	row := sidebarLine(t, m, "staging")
	next, _ := m.Update(click(3, row))
	if got := next.(appModel).activeEnv; got != 1 {
		t.Fatalf("click on staging: activeEnv = %d", got)
	}
}

// The wheel scrolls lists and views instead of being ignored.
func TestMouseWheelScrollsDashboard(t *testing.T) {
	m := newAppModel(nil, t.TempDir())
	m.splash = false
	m.width, m.height = 120, 20
	m.currentView = viewDashboard
	m.dashboard.loaded = true
	m.dashboard.server = &versassh.ServerInfo{}
	m.dashboard.alerts = make([]string, 40) // taller than the pane
	for i := range m.dashboard.alerts {
		m.dashboard.alerts[i] = "alert"
	}
	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if next.(appModel).dashboard.offset == 0 {
		t.Fatal("wheel down didn't scroll the dashboard")
	}
}

// sidebarLine is the screen row where the sidebar draws text.
func sidebarLine(t *testing.T, m appModel, text string) int {
	t.Helper()
	for i, line := range strings.Split(m.View(), "\n") {
		if side := ansi.Strip(line); len(side) > 0 && strings.Contains(ansi.Truncate(side, sidebarRenderedWidth(), ""), text) {
			return i
		}
	}
	t.Fatalf("%q not in the sidebar:\n%s", text, m.View())
	return 0
}

// Every deploy file is listed with its environments; picking one of another file (click
// or ↓ from the sidebar) loads that file with that environment.
func TestSidebarListsEveryFileAndSwitches(t *testing.T) {
	dir := t.TempDir()
	write := func(name, envs string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("project: app\nenvironments:\n"+envs), 0o644)
		return p
	}
	key := filepath.Join(dir, "id")
	os.WriteFile(key, []byte("x"), 0o600)
	env := func(n string) string {
		return "  " + n + ":\n    ssh: { host: 127.0.0.1, user: deploy, key_path: " + key + " }\n    remote_path: /var/www/app\n    builds: { php: { enabled: true } }\n"
	}
	main := write("deploy.yml", env("production"))
	docker := write("deploy.docker.yml", env("docker")+env("qa"))
	t.Chdir(dir)

	cfg, err := config.Load(main)
	if err != nil {
		t.Fatal(err)
	}
	m := newAppModel(cfg, dir)
	m.discoverConfigs()
	m.useConfig(main, true)
	m.splash, m.width, m.height = false, 170, 40
	for _, s := range []string{"deploy.yml", "production", "deploy.docker", "docker", "qa"} {
		sidebarLine(t, m, s)
	}

	next, _ := m.Update(click(3, sidebarLine(t, m, "qa")))
	m = next.(appModel)
	if m.config.configPath != docker || m.activeEnvName() != "qa" {
		t.Fatalf("click on qa: file %s, env %q", m.config.configPath, m.activeEnvName())
	}

	// keyboard (Tab focuses the sidebar): files are sorted, deploy.docker.yml first;
	// ↓ from qa crosses into deploy.yml, ↑ comes back
	m.sidebarFocused = true
	for _, step := range []struct {
		key       tea.KeyType
		env, file string
	}{{tea.KeyUp, "docker", docker}, {tea.KeyDown, "qa", docker}, {tea.KeyDown, "production", main}, {tea.KeyUp, "qa", docker}} {
		next, _ = m.Update(tea.KeyMsg{Type: step.key})
		m = next.(appModel)
		if m.activeEnvName() != step.env || m.config.configPath != step.file {
			t.Fatalf("key %v: %s/%s, want %s/%s", step.key, filepath.Base(m.config.configPath), m.activeEnvName(), filepath.Base(step.file), step.env)
		}
	}
}

// ←/→ reach Switch Config (multi-deploy) and leave it again.
func TestArrowsReachSwitchConfig(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.Environment{"production": {}}}
	m := newAppModel(cfg, t.TempDir())
	m.splash = false
	m.currentView = viewDashboard
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := next.(appModel).currentView; got != viewConfigSelector {
		t.Fatalf("← from Dashboard: %v", got)
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(appModel).currentView; got != viewDashboard {
		t.Fatalf("→ from Switch Config: %v", got)
	}
	m.currentView = viewConfig
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(appModel).currentView; got != viewConfigSelector {
		t.Fatalf("→ from Config: %v", got)
	}
}

// Startup with several deploy files and no deploy.yml (the picker): no phantom
// deploy.yml in the sidebar; Tab lands on the first environment and opens its Dashboard,
// ↓ moves to the next file, Enter leaves the sidebar; a click from the picker opens the
// Dashboard too.
func TestSidebarFromThePicker(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	os.WriteFile(key, []byte("x"), 0o600)
	for name, env := range map[string]string{"deploy.docker.yml": "docker", "versa_deploy_vserv.yml": "production"} {
		os.WriteFile(filepath.Join(dir, name), []byte("project: app\nenvironments:\n  "+env+":\n    ssh: { host: 127.0.0.1, user: deploy, key_path: "+key+" }\n    remote_path: /var/www/app\n    builds: { php: { enabled: true } }\n"), 0o644)
	}
	t.Chdir(dir)
	start := func() appModel {
		m := newAppModel(nil, dir)
		m.discoverConfigs()
		m.useConfig("", false)
		m.splash, m.width, m.height = false, 170, 40
		return m
	}
	m := start()
	if m.currentView != viewConfigSelector {
		t.Fatalf("startup view %v, want the picker", m.currentView)
	}
	for _, r := range m.sidebarRows() {
		if r.env == "" && filepath.Base(r.file) == "deploy.yml" {
			t.Fatal("a deploy.yml that doesn't exist is listed")
		}
	}
	press := func(k tea.KeyType) { n, _ := m.Update(tea.KeyMsg{Type: k}); m = n.(appModel) }
	press(tea.KeyTab)
	if m.activeEnvName() != "docker" || m.currentView != viewDashboard || !m.sidebarFocused {
		t.Fatalf("Tab: env %q view %v focused %v", m.activeEnvName(), m.currentView, m.sidebarFocused)
	}
	press(tea.KeyDown)
	if m.activeEnvName() != "production" || filepath.Base(m.config.configPath) != "versa_deploy_vserv.yml" {
		t.Fatalf("↓: %s/%s", filepath.Base(m.config.configPath), m.activeEnvName())
	}
	press(tea.KeyEnter)
	if m.sidebarFocused {
		t.Fatal("Enter must leave the sidebar")
	}

	m = start()
	n, _ := m.Update(click(3, sidebarLine(t, m, "production")))
	if m = n.(appModel); m.currentView != viewDashboard || m.activeEnvName() != "production" {
		t.Fatalf("click from the picker: view %v env %q", m.currentView, m.activeEnvName())
	}
	// a connection opened for the file the user left is dropped, not given to this one
	other := m.cfg
	m.loadConfigFile(filepath.Join(dir, "deploy.docker.yml"), "")
	n, _ = m.Update(msgConnError{envName: "docker", err: os.ErrClosed, cfg: other})
	if m = n.(appModel); m.connStates["docker"] == connError {
		t.Fatal("a stale connection result changed the new file's environment")
	}
}
