// Package tui provides an interactive terminal UI for versaDeploy.
// Launch it with `versa --gui`.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
)

// Launch starts the interactive TUI. It blocks until the user quits. configPath is the
// file cfg was loaded from ("" when none): the Config and Services views write to it.
// explicit means the user chose it (--config): no config picker then.
func Launch(cfg *config.Config, repoPath, configPath string, explicit bool) error {
	m := newAppModel(cfg, repoPath)
	m.discoverConfigs()
	m.useConfig(configPath, explicit)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
