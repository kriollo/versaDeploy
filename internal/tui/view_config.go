package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type configModel struct {
	configPath string
	content    string
	loaded     bool
	loading    bool
	err        error

	isEditing  bool
	cursorLine int
	cursorCol  int
	hasChanges bool
	viewStart  int
	contentH   int // actual available content height, set from WindowSizeMsg
}

func newConfigModel(repoPath string) configModel {
	configPath := "deploy.yml"
	if repoPath != "" {
		configPath = repoPath + "/deploy.yml"
	}

	m := configModel{
		configPath: configPath,
	}
	return m
}

func (m *configModel) load() tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(m.configPath)
		if err != nil {
			return msgConfigLoaded{err: err}
		}
		return msgConfigLoaded{content: string(data)}
	}
}

func (m *configModel) save() tea.Cmd {
	content := m.content
	return func() tea.Msg {
		err := os.WriteFile(m.configPath, []byte(content), 0644)
		return msgConfigSaved{err: err}
	}
}

type msgConfigLoaded struct {
	content string
	err     error
}

type msgConfigSaved struct {
	err error
}

func (m *configModel) applyLoaded(msg msgConfigLoaded) {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		m.loaded = false
		return
	}
	m.content = msg.content
	m.loaded = true
	m.err = nil
	m.viewStart = 0
	m.cursorLine = 0
	m.cursorCol = 0
}

func (m *configModel) applySaved(msg msgConfigSaved) {
	if msg.err != nil {
		m.err = msg.err
		return
	}
	m.hasChanges = false
	m.err = nil
}

func (m *configModel) view(width int) string {
	if m.loading {
		return StyleMuted.Render("Loading config...")
	}

	if m.err != nil && !m.loaded {
		errMsg := StyleError.Render(fmt.Sprintf("Error: %v", m.err))
		hint := StyleMuted.Render("\n\nMake sure deploy.yml exists in the project root.")
		return errMsg + hint
	}

	var status string
	if m.isEditing {
		status = StyleWarning.Render(" [EDIT: arrows/move  type/insert  Ctrl+S/save  Esc/cancel] ")
	} else if m.hasChanges {
		status = StyleWarning.Render(" [modified — unsaved changes] ")
	} else if m.err != nil {
		status = StyleError.Render(fmt.Sprintf(" [%v]", m.err))
	} else {
		status = StyleMuted.Render(" [e=edit  r=reload  Esc=back  ←/→=switch views] ")
	}

	header := StyleHeader.Width(width).Render("Config: "+m.configPath) + "\n"

	contentW := width - 10
	if contentW < 20 {
		contentW = 20
	}
	// Use actual terminal content height if available, else fallback to a safe default
	contentH := m.contentH - 3 // subtract header + footer rows
	if contentH < 5 {
		contentH = 20
	}

	lines := strings.Split(m.content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Clamp cursor
	if m.cursorLine >= len(lines) {
		m.cursorLine = len(lines) - 1
	}
	if m.cursorLine < 0 {
		m.cursorLine = 0
	}
	if m.cursorCol < 0 {
		m.cursorCol = 0
	}
	if m.cursorCol > len(lines[m.cursorLine]) {
		m.cursorCol = len(lines[m.cursorLine])
	}

	maxViewStart := len(lines) - contentH
	if maxViewStart < 0 {
		maxViewStart = 0
	}
	if m.viewStart > maxViewStart {
		m.viewStart = maxViewStart
	}
	if m.viewStart < 0 {
		m.viewStart = 0
	}

	// Scroll only when cursor leaves the visible window (1-line scroll, never jumps)
	visibleEnd := m.viewStart + contentH - 1
	if m.cursorLine > visibleEnd {
		m.viewStart += m.cursorLine - visibleEnd // scroll down by the overshoot
		if m.viewStart > maxViewStart {
			m.viewStart = maxViewStart
		}
	} else if m.cursorLine < m.viewStart {
		m.viewStart -= m.viewStart - m.cursorLine // scroll up by the overshoot
		if m.viewStart < 0 {
			m.viewStart = 0
		}
	}

	visibleStart := m.viewStart
	visibleEnd = m.viewStart + contentH
	if visibleEnd > len(lines) {
		visibleEnd = len(lines)
	}

	var bodyLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		lineNum := fmt.Sprintf("%4d ", i+1)
		lineContent := lines[i]

		if m.isEditing && i == m.cursorLine {
			col := m.cursorCol
			if col > len(lineContent) {
				col = len(lineContent)
			}
			if col < 0 {
				col = 0
			}
			before := lineContent[:col]
			char := lineContent[col:]
			if len(char) > 0 {
				lineContent = before + StyleSelected.Render(string(char[0])) + char[1:]
			} else {
				lineContent = before + StyleSelected.Render(" ")
			}
		} else if !m.isEditing && i == m.cursorLine {
			// Highlight current line in read-only mode too
			lineContent = StyleActive.Render(lineContent)
		}

		if len(lineContent) > contentW-5 {
			// Only truncate plain strings (styled ones may have escape codes)
			if !strings.Contains(lineContent, "\x1b") {
				lineContent = lineContent[:contentW-8] + "..."
			}
		}

		bodyLines = append(bodyLines, lineNum+lineContent)
	}

	for len(bodyLines) < contentH {
		bodyLines = append(bodyLines, strings.Repeat(" ", contentW))
	}

	// Scrollbar hint
	scrollInfo := ""
	if len(lines) > contentH {
		pct := 0
		if maxViewStart > 0 {
			pct = int(float64(m.viewStart) / float64(maxViewStart) * 100)
		}
		scrollInfo = fmt.Sprintf("  line %d/%d  %d%%", m.cursorLine+1, len(lines), pct)
	}
	footerWithScroll := StyleSurface.Width(width).Render(status + StyleMuted.Render(scrollInfo))

	body := lipgloss.JoinVertical(lipgloss.Left, bodyLines...)

	return header + body + "\n" + footerWithScroll
}


func (m *configModel) handleKey(msg tea.Msg) ([]tea.Cmd, bool) {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			if m.loaded && !m.isEditing {
				m.isEditing = true
				return cmds, true
			}
		case "ctrl+s":
			if m.isEditing {
				m.isEditing = false
				cmds = append(cmds, m.save())
				return cmds, true
			}
		case "esc":
			if m.isEditing {
				m.isEditing = false
				m.cursorLine = 0
				m.cursorCol = 0
				return cmds, true
			}
		case "r":
			if m.loaded && !m.isEditing {
				cmds = append(cmds, m.load())
				return cmds, true
			}
		}

		if m.isEditing {
			m.handleEditKey(msg)
			return cmds, true
		}

		switch msg.String() {
		case "up", "k":
			if m.cursorLine > 0 {
				m.cursorLine--
			}
		case "down", "j":
			lines := strings.Split(m.content, "\n")
			if m.cursorLine < len(lines)-1 {
				m.cursorLine++
			}
		case "pgup":
			if m.cursorLine >= 10 {
				m.cursorLine -= 10
			} else {
				m.cursorLine = 0
			}
		case "pgdown":
			lines := strings.Split(m.content, "\n")
			if m.cursorLine+10 < len(lines) {
				m.cursorLine += 10
			} else {
				m.cursorLine = len(lines) - 1
			}
		case "home":
			m.cursorLine = 0
		case "end":
			lines := strings.Split(m.content, "\n")
			m.cursorLine = len(lines) - 1
		}

		return cmds, true
	}

	return cmds, false
}

func (m *configModel) handleEditKey(msg tea.KeyMsg) {
	lines := strings.Split(m.content, "\n")
	if m.cursorLine >= len(lines) {
		m.cursorLine = len(lines) - 1
	}
	if m.cursorLine < 0 {
		m.cursorLine = 0
	}

	currentLine := lines[m.cursorLine]
	if m.cursorCol > len(currentLine) {
		m.cursorCol = len(currentLine)
	}
	if m.cursorCol < 0 {
		m.cursorCol = 0
	}

	switch msg.String() {
	case "up", "ctrl+p":
		if m.cursorLine > 0 {
			m.cursorLine--
			if m.cursorCol > len(lines[m.cursorLine]) {
				m.cursorCol = len(lines[m.cursorLine])
			}
		}
	case "down", "ctrl+n":
		if m.cursorLine < len(lines)-1 {
			m.cursorLine++
			if m.cursorCol > len(lines[m.cursorLine]) {
				m.cursorCol = len(lines[m.cursorLine])
			}
		}
	case "left", "ctrl+b":
		if m.cursorCol > 0 {
			m.cursorCol--
		} else if m.cursorLine > 0 {
			m.cursorLine--
			m.cursorCol = len(lines[m.cursorLine])
		}
	case "right", "ctrl+f":
		if m.cursorCol < len(currentLine) {
			m.cursorCol++
		} else if m.cursorLine < len(lines)-1 {
			m.cursorLine++
			m.cursorCol = 0
		}
	case "home", "ctrl+a":
		m.cursorCol = 0
	case "end", "ctrl+e":
		m.cursorCol = len(currentLine)
	case "backspace":
		if m.cursorCol > 0 {
			currentLine = currentLine[:m.cursorCol-1] + currentLine[m.cursorCol:]
			lines[m.cursorLine] = currentLine
			m.cursorCol--
			m.hasChanges = true
		} else if m.cursorLine > 0 {
			prevLen := len(lines[m.cursorLine-1])
			lines[m.cursorLine-1] = lines[m.cursorLine-1] + currentLine
			lines = append(lines[:m.cursorLine], lines[m.cursorLine+1:]...)
			m.cursorLine--
			m.cursorCol = prevLen
			m.hasChanges = true
		}
	case "del":
		if m.cursorCol < len(currentLine) {
			currentLine = currentLine[:m.cursorCol] + currentLine[m.cursorCol+1:]
			lines[m.cursorLine] = currentLine
			m.hasChanges = true
		} else if m.cursorLine < len(lines)-1 {
			lines[m.cursorLine] = currentLine + lines[m.cursorLine+1]
			lines = append(lines[:m.cursorLine+1], lines[m.cursorLine+2:]...)
			m.hasChanges = true
		}
	case "enter":
		lines = append(lines[:m.cursorLine], append([]string{"", currentLine[m.cursorCol:]}, lines[m.cursorLine+1:]...)...)
		lines[m.cursorLine] = currentLine[:m.cursorCol]
		m.cursorLine++
		m.cursorCol = 0
		m.hasChanges = true
	case "tab":
		spaces := "  "
		currentLine = currentLine[:m.cursorCol] + spaces + currentLine[m.cursorCol:]
		lines[m.cursorLine] = currentLine
		m.cursorCol += len(spaces)
		m.hasChanges = true
	default:
		ch := msg.String()
		if len(ch) == 1 && ch >= " " && ch <= "~" {
			currentLine = currentLine[:m.cursorCol] + ch + currentLine[m.cursorCol:]
			lines[m.cursorLine] = currentLine
			m.cursorCol++
			m.hasChanges = true
		}
	}

	m.content = strings.Join(lines, "\n")
}
