package tui

import "github.com/charmbracelet/lipgloss"

func renderStatusbar(width int, hints string, extra string) string {
	left := StyleMuted.Render(hints)
	right := StyleMuted.Render(extra)
	pad := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if pad < 0 {
		pad = 0
	}
	content := left
	if extra != "" {
		for i := 0; i < pad; i++ {
			content += " "
		}
		content += right
	}
	return StyleStatusbar.Width(width).Render(content)
}
