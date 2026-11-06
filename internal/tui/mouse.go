package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen layout the mouse maps onto (see appModel.View): row 0 is the header; from
// row 1 the sidebar (title, separator, one row per environment) sits left of the
// content pane, whose first row is the tab bar.
const (
	tabBarRow   = 1
	firstEnvRow = 3
)

// handleMouse: wheel scrolls the current view, a left click on a tab switches view,
// on an environment selects it.
func (m appModel) handleMouse(msg tea.MouseMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if m.splash {
		return m, tea.Batch(cmds...)
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
		return m.wheel(msg.Button == tea.MouseButtonWheelUp, cmds)
	case msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress:
		return m, tea.Batch(cmds...)
	}

	// modal states own the keyboard: clicks don't navigate away from them
	busy := (m.currentView == viewOperations && m.operations.running) ||
		(m.currentView == viewServices && (m.services.running || m.services.editing || m.services.confirmTitle != "")) ||
		(m.currentView == viewConfig && m.config.isEditing) ||
		(m.currentView == viewTerminal && m.terminal.running)
	if busy {
		return m, tea.Batch(cmds...)
	}

	sideW := sidebarRenderedWidth()
	if msg.X >= sideW && msg.Y == tabBarRow {
		if v, ok := tabAt(msg.X - sideW); ok && v != m.currentView {
			m.sidebarFocused = false
			if v == viewConfigSelector {
				m.currentView = v
			} else {
				cmds = append(cmds, m.switchToView(v)...)
			}
		}
		return m, tea.Batch(cmds...)
	}
	if msg.X < sideW {
		if i := msg.Y - firstEnvRow; i >= 0 && i < len(m.envNames) && i != m.activeEnv {
			m.activeEnv = i
			m = m.resetViewState()
			cmds = append(cmds, m.autoConnectCmds()...)
		}
	}
	return m, tea.Batch(cmds...)
}

// wheel scrolls the view under the cursor: viewports directly, lists by moving
// their selection like ↑/↓.
func (m appModel) wheel(up bool, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case viewTerminal:
		if up {
			m.terminal.vp.LineUp(3)
		} else {
			m.terminal.vp.LineDown(3)
		}
	case viewServices:
		if up {
			m.services.vp.LineUp(3)
		} else {
			m.services.vp.LineDown(3)
		}
	case viewOperations:
		if up {
			m.operations.scrollUp()
		} else {
			m.operations.scrollDown()
		}
	case viewDashboard:
		step := 3
		if up {
			step = -3
		}
		m.dashboard.scroll(step, m.contentHeight())
	default:
		if m.sidebarFocused || (m.currentView == viewConfig && m.config.isEditing) {
			return m, tea.Batch(cmds...)
		}
		k := tea.KeyMsg{Type: tea.KeyDown}
		if up {
			k = tea.KeyMsg{Type: tea.KeyUp}
		}
		return m.handleKey(k, cmds)
	}
	return m, tea.Batch(cmds...)
}

// tabAt maps a column inside the content pane to the tab drawn there (renderTabBar:
// one cell of padding, then " name " labels joined by "│").
func tabAt(x int) (viewID, bool) {
	x-- // StyleSurface left padding
	for i, v := range viewLabels {
		w := lipgloss.Width(" " + v.name + " ")
		if x >= 0 && x < w {
			return viewID(i), true
		}
		x -= w + 1
	}
	return 0, false
}

func sidebarRenderedWidth() int {
	return lipgloss.Width(strings.SplitN(renderSidebar(3, nil, 0, false, nil), "\n", 2)[0])
}
