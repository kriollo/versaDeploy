package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
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
	editor     textarea.Model // the same editor as Files/Shared (runes, paste, ctrl+k…)
	cursorLine int
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
		status = StyleWarning.Render(" [EDIT: Ctrl+S save · Esc cancel · Ctrl+K cut to line end · Ctrl+Home/End top/bottom] ")
	} else if m.hasChanges {
		status = StyleWarning.Render(" [modified — unsaved changes] ")
	} else if m.err != nil {
		status = StyleError.Render(fmt.Sprintf(" [%v]", m.err))
	} else {
		status = StyleMuted.Render(" [e=edit  r=reload  Esc=back  ←/→=switch views] ")
	}

	header := StyleHeader.Width(width).Render("Config: "+m.configPath) + "\n"
	if m.isEditing {
		h := m.contentH - 3
		if h < 5 {
			h = 20
		}
		m.editor.SetWidth(width - 4)
		m.editor.SetHeight(h)
		return header + m.editor.View() + "\n" + StyleSurface.Width(width).Render(status)
	}

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

	// Keep a small vertical margin around the cursor to avoid aggressive follow-scroll.
	// This makes upward navigation near the file end easier to read.
	topMargin := 2
	bottomMargin := 2
	if contentH < 8 {
		topMargin = 0
		bottomMargin = 0
	}

	visibleTop := m.viewStart + topMargin
	visibleBottom := m.viewStart + contentH - 1 - bottomMargin

	if m.cursorLine > visibleBottom {
		m.viewStart = m.cursorLine - (contentH - 1 - bottomMargin)
		if m.viewStart > maxViewStart {
			m.viewStart = maxViewStart
		}
	} else if m.cursorLine < visibleTop {
		m.viewStart = m.cursorLine - topMargin
		if m.viewStart < 0 {
			m.viewStart = 0
		}
	}

	visibleStart := m.viewStart
	visibleEnd := m.viewStart + contentH
	if visibleEnd > len(lines) {
		visibleEnd = len(lines)
	}

	var bodyLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		lineNum := fmt.Sprintf("%4d ", i+1)
		lineContent := lines[i]

		if i == m.cursorLine {
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
				m.editor = textarea.New()
				m.editor.ShowLineNumbers = true
				m.editor.SetValue(m.content)
				m.editor.Focus()
				m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlHome}) // start at the first line
				m.isEditing = true
				return cmds, true
			}
		case "ctrl+s":
			if m.isEditing {
				m.isEditing = false
				m.content = m.editor.Value()
				cmds = append(cmds, m.save())
				return cmds, true
			}
		case "esc":
			if m.isEditing {
				m.isEditing = false // discard the edit
				return cmds, true
			}
		case "r":
			if m.loaded && !m.isEditing {
				cmds = append(cmds, m.load())
				return cmds, true
			}
		}

		if m.isEditing {
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			m.hasChanges = m.editor.Value() != m.content
			return append(cmds, cmd), true
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
