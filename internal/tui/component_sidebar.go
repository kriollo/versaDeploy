package tui

import (
	"fmt"
	"strings"
)

const sidebarWidth = 18

func renderSidebar(height int, envNames []string, activeEnv int, focused bool, states map[string]connState) string {
	title := StyleTableHeader.Render("ENVIRONMENTS")

	var rows []string
	rows = append(rows, title)
	rows = append(rows, StyleMuted.Render(strings.Repeat("─", sidebarWidth-2)))

	for i, name := range envNames {
		indicator := connIndicator(states[name])
		label := name
		if len(label) > sidebarWidth-5 {
			label = label[:sidebarWidth-5]
		}

		line := fmt.Sprintf("%s %s", indicator, label)
		if i == activeEnv {
			if focused {
				line = StyleSelected.Render(fmt.Sprintf(" %s ", line))
			} else {
				line = StyleActive.Render(fmt.Sprintf("%s %s", indicator, label))
			}
		} else {
			line = StyleMuted.Render(fmt.Sprintf("%s %s", indicator, label))
		}
		rows = append(rows, line)
	}

	// Pad to fill height
	for len(rows) < height-2 {
		rows = append(rows, "")
	}

	return StyleSidebar.
		Width(sidebarWidth).
		Height(height).
		Render(strings.Join(rows, "\n"))
}

func connIndicator(s connState) string {
	switch s {
	case connConnected:
		return StyleConnected.Render("●")
	case connConnecting:
		return StyleConnecting.Render("◌")
	case connError:
		return StyleErrorState.Render("✕")
	default:
		return StyleDisconnected.Render("○")
	}
}
