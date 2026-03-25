package tui

import "github.com/charmbracelet/lipgloss"

// Color palette (Tokyo Night)
var (
	colorBg      = lipgloss.Color("#1a1b26")
	colorSurface = lipgloss.Color("#24283b")
	colorAccent  = lipgloss.Color("#7aa2f7")
	colorSuccess = lipgloss.Color("#9ece6a")
	colorWarning = lipgloss.Color("#e0af68")
	colorError   = lipgloss.Color("#f7768e")
	colorMuted   = lipgloss.Color("#565f89")
	colorText    = lipgloss.Color("#c0caf5")
)

// Icon color constants — language brand colors
var (
	iconColorGo      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ADD8"))
	iconColorPython  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3776AB"))
	iconColorJS      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DF1E"))
	iconColorTS      = lipgloss.NewStyle().Foreground(lipgloss.Color("#3178C6"))
	iconColorPHP     = lipgloss.NewStyle().Foreground(lipgloss.Color("#777BB4"))
	iconColorRust    = lipgloss.NewStyle().Foreground(lipgloss.Color("#DEA584"))
	iconColorRuby    = lipgloss.NewStyle().Foreground(lipgloss.Color("#CC342D"))
	iconColorFolder  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	iconColorConfig  = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	iconColorLock    = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	iconColorImage   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	iconColorArchive = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
)

var (
	StyleHeader = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorText).
			Bold(true).
			Padding(0, 1)

	StyleTitle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	StyleSurface = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorText).
			Padding(0, 1)

	StyleSidebar = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorText).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	StyleContent = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	StyleStatusbar = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorMuted).
			Padding(0, 1)

	StyleActive = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	StyleError = lipgloss.NewStyle().
			Foreground(colorError)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleSelected = lipgloss.NewStyle().
			Background(colorAccent).
			Foreground(colorBg).
			Bold(true)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted)

	StyleTableHeader = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	StyleConnected    = lipgloss.NewStyle().Foreground(colorSuccess)
	StyleConnecting   = lipgloss.NewStyle().Foreground(colorWarning)
	StyleDisconnected = lipgloss.NewStyle().Foreground(colorMuted)
	StyleErrorState   = lipgloss.NewStyle().Foreground(colorError)

	StyleSection = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	StyleCmd = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	StyleHint = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)
)
