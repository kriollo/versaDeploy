package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderStatusbar is always one line: a long status (an SSH error) wrapped onto two or
// three lines used to push the whole view up and hide the header. The status wins:
// the key hints are cut first, then the status itself, with "…".
func renderStatusbar(width int, hints string, extra string) string {
	avail := width - 2
	if avail < 1 {
		avail = 1
	}
	extra = ansi.Truncate(extra, avail, "…")
	hintsW := avail - lipgloss.Width(extra) - 1
	if extra == "" {
		hintsW = avail
	}
	if hintsW < 1 {
		hints = ""
	} else {
		hints = ansi.Truncate(hints, hintsW, "…")
	}
	left := StyleMuted.Render(hints)
	right := StyleMuted.Render(extra)
	pad := avail - lipgloss.Width(left) - lipgloss.Width(right)
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
	return StyleStatusbar.Width(width).MaxHeight(1).Render(content)
}
