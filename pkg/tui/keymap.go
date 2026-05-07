package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Home        key.Binding
	End         key.Binding
	Mark        key.Binding
	Kill        key.Binding
	Refresh     key.Binding
	DetailPane  key.Binding
	OpenBrowser key.Binding
	ClearMarks  key.Binding
	AutoRefresh key.Binding
	Quit        key.Binding
	Help        key.Binding
	Back        key.Binding
}

func NewKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "bottom"),
		),
		Mark: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "mark"),
		),
		Kill: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "kill"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		DetailPane: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "detail"),
		),
		OpenBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		ClearMarks: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "clear marks"),
		),
		AutoRefresh: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "auto-rfrsh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Mark, k.Kill, k.DetailPane, k.OpenBrowser, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Mark, k.ClearMarks, k.Kill, k.OpenBrowser},
		{k.DetailPane, k.Refresh, k.AutoRefresh, k.Help, k.Quit},
	}
}
