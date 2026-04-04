package dashboard

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Tab           key.Binding
	ShiftTab      key.Binding
	Refresh       key.Binding
	Focus         key.Binding
	New           key.Binding
	Delete        key.Binding
	Enter         key.Binding
	ServicesUp    key.Binding
	ServicesDown  key.Binding
	AttachService key.Binding
	Quit          key.Binding
}

var defaultKeys = keyMap{
	Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Tab:           key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
	ShiftTab:      key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev panel")),
	Refresh:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Focus:         key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "focus")),
	New:           key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new worktree")),
	Delete:        key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "clean worktree")),
	Enter:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "go to worktree")),
	ServicesUp:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "start services")),
	ServicesDown:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop services")),
	AttachService: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "attach to service")),
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
