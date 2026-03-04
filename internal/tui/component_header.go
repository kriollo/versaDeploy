package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHeader(width int, version, activeEnv string, connected bool) string {
	left := StyleTitle.Render("versaDeploy") + StyleMuted.Render(" "+version)

	envPart := StyleMuted.Render("[ENV: ") + StyleActive.Render(activeEnv) + StyleMuted.Render("]")

	var connPart string
	if connected {
		connPart = StyleConnected.Render("● CONNECTED")
	} else {
		connPart = StyleDisconnected.Render("○ DISCONNECTED")
	}

	leftLen := lipgloss.Width(left)
	envLen := lipgloss.Width(envPart)
	connLen := lipgloss.Width(connPart)
	pad := width - leftLen - envLen - connLen - 4
	if pad < 1 {
		pad = 1
	}

	content := fmt.Sprintf("%s%s%s  %s",
		left,
		strings.Repeat(" ", pad),
		envPart,
		connPart,
	)
	return StyleHeader.Width(width).Render(content)
}
