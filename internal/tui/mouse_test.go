package tui

import (
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
	row := firstEnvRow + 1 // envNames are sorted: production, staging
	if line := ansi.Strip(strings.Split(m.View(), "\n")[row]); !strings.Contains(line, "staging") {
		t.Fatalf("row %d is not the staging environment: %q", row, line)
	}
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
