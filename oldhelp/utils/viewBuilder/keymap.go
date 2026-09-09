package viewBuilder

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Help  key.Binding
	Quit  key.Binding
}

var Defaultkeys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("e", "up"),
		key.WithHelp("e/↑", "Move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("d", "down"),
		key.WithHelp("d/↓", "Move down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("↵", "Select option"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "Toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "Back/Quit"),
	),
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Help, k.Quit},
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}
