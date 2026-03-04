package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the TUI.
type KeyMap struct {
	View1    key.Binding
	View2    key.Binding
	View3    key.Binding
	View4    key.Binding
	View5    key.Binding
	Tab      key.Binding
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Deploy   key.Binding
	Rollback key.Binding
	Back     key.Binding
	Connect  key.Binding
	Quit     key.Binding
	Help     key.Binding
}

// Keys is the global keybinding map.
var Keys = KeyMap{
	View1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
	View2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "releases")),
	View3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "files")),
	View4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "shared")),
	View5:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "deploy")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "sidebar")),
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "select")),
	Deploy:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "deploy")),
	Rollback: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rollback")),
	Back:     key.NewBinding(key.WithKeys("backspace"), key.WithHelp("⌫", "back")),
	Connect:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// ShortHelp returns the abbreviated key hints shown in the status bar.
func (k KeyMap) ShortHelp() string {
	return "Tab:sidebar  1-5:views  d:deploy  r:rollback  c:connect  q:quit  ?:help"
}
