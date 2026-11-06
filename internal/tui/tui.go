// Package tui provides an interactive terminal UI for versaDeploy.
// Launch it with `versa --gui`.
package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
)

// Launch starts the interactive TUI. It blocks until the user quits. configPath is the
// file cfg was loaded from ("" when none): the Config and Services views write to it.
func Launch(cfg *config.Config, repoPath, configPath string) error {
	m := newAppModel(cfg, repoPath)
	m.discoverConfigs()
	if cfg != nil && configPath != "" {
		if abs, err := filepath.Abs(configPath); err == nil {
			configPath = abs
		}
		m.config.configPath = configPath
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
