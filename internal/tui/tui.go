// Package tui provides an interactive terminal UI for versaDeploy.
// Launch it with `versa --gui`.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
)

// Launch starts the interactive TUI. It blocks until the user quits.
func Launch(cfg *config.Config, repoPath string) error {
	m := newAppModel(cfg, repoPath)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
