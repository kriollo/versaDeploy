package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The bitmap in arts/ is a PNG/WebP. Terminals do not share one image
// protocol (sixel, kitty, iTerm), so the TUI redraws the logo with text in
// the logo's own colors: a blue/orange/green arrow ring around a server rack
// with a ">_" terminal and a lock, over the gradient wordmark.

var (
	logoBlue   = lipgloss.Color("#2f80d8")
	logoTeal   = lipgloss.Color("#3fa7c9")
	logoGreen  = lipgloss.Color("#6cc04a")
	logoOrange = lipgloss.Color("#f39c26")
	logoSteel  = lipgloss.Color("#8fa8c4")
	logoLock   = lipgloss.Color("#c8d6e5")
)

// wordmarkColors follows the logo letter by letter: "versa" blue→teal,
// "Deplo" green, "y" orange.
var wordmarkColors = []lipgloss.Color{
	logoBlue, logoBlue, logoBlue, logoTeal, logoTeal,
	logoGreen, logoGreen, logoGreen, logoGreen, logoGreen,
	logoOrange,
}

func brandTitle() string {
	var b strings.Builder
	for i, r := range "versaDeploy" {
		b.WriteString(lipgloss.NewStyle().Foreground(wordmarkColors[i]).Bold(true).Render(string(r)))
	}
	return b.String()
}

// brandMark is the one-line logo used in the header.
func brandMark() string {
	return lipgloss.NewStyle().Foreground(logoOrange).Bold(true).Render("↻ ") + brandTitle()
}

const brandBannerWidth = 28

// brandBanner is the logo for tall dashboards. Every line is padded to
// brandBannerWidth so the art stays aligned when centered.
func brandBanner() []string {
	fg := func(c lipgloss.Color) func(...string) string {
		return lipgloss.NewStyle().Foreground(c).Render
	}
	b, o, g, s := fg(logoBlue), fg(logoOrange), fg(logoGreen), fg(logoSteel)
	led := fg(logoGreen)
	term := fg(logoTeal)
	prompt := lipgloss.NewStyle().Foreground(logoLock).Bold(true).Render
	lock := fg(logoLock)

	lines := []string{
		b("  ▲ ") + o("━━━━━━━━━━━━━━━━━━━━━┓"),
		b("  ┃ ") + s("  ╭──────────────╮") + o("   ▼"),
		b("  ┃ ") + s("  │ ") + led("●●●") + s("  ═══════ │") + g("   ┃"),
		b("  ┃ ") + s("  ├──────────────┤") + g("   ┃"),
		b("  ┃ ") + s("  │ ") + led("●●●") + s(" ") + term("╭─────────╮") + g("  ┃"),
		b("  ┃ ") + s("  ├─────") + term("│ ") + prompt(">_") + term("   ") + lock("╭╮") + term(" │") + g("  ┃"),
		b("  ┃ ") + s("  │ ") + led("●●●") + s(" ") + term("│      ") + lock("██") + term(" │") + g("  ┃"),
		b("  ┃ ") + s("  ╰─────") + term("╰─────────╯") + g("  ┃"),
		b("  ┗━━━━━━━━━") + g("  ◀━━━━━━━━━━┛"),
		"",
		centerLine(brandTitle(), brandBannerWidth),
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w < brandBannerWidth {
			lines[i] = l + strings.Repeat(" ", brandBannerWidth-w)
		}
	}
	return lines
}

func centerLine(line string, width int) string {
	w := lipgloss.Width(line)
	if w >= width {
		return line
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + line + strings.Repeat(" ", width-w-left)
}
