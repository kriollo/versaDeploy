package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the TUI.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Tab      key.Binding
	Deploy   key.Binding
	Rollback key.Binding
	Back     key.Binding
	Connect  key.Binding
	Quit     key.Binding
	Help     key.Binding
	Edit     key.Binding
	Save     key.Binding
	Refresh  key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
}

// Keys is the global keybinding map.
var Keys = KeyMap{
	Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
	Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
	Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "prev view")),
	Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next view")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "select/toggle")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle sidebar")),
	Deploy:   key.NewBinding(key.WithKeys("d"), key.WithHelp("D", "deploy")),
	Rollback: key.NewBinding(key.WithKeys("r"), key.WithHelp("R", "rollback")),
	Back:     key.NewBinding(key.WithKeys("backspace"), key.WithHelp("⌫", "go up")),
	Connect:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Save:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("Ctrl+S", "save")),
	Refresh:  key.NewBinding(key.WithKeys("f5"), key.WithHelp("F5", "refresh")),
	PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Home:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "go to start")),
	End:      key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "go to end")),
}

// ShortHelp returns the key hints displayed in the status bar.
func (k KeyMap) ShortHelp() string {
	return "←/→:views  Tab:sidebar  ↑/↓:navigate  ↵:select  F5:refresh  D:deploy  R:rollback  c:connect  q:quit"
}
