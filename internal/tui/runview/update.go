package runview

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	switch msg.String() {
	case keyQuit, keyInterrupt:
		return m.detach()
	case keyUp, keyVimUp:
		return m.move(-1)
	case keyDown, keyVimDown:
		return m.move(1)
	case keyFilter:
		m.filtering = true
		return m, nil
	case keyRefresh:
		return m, m.refreshCmd()
	case keyPageUp:
		return m.scroll(m.layout().PaneRows), nil
	case keyPageDown:
		return m.scroll(-m.layout().PaneRows), nil
	case keyScrollUp:
		return m.scroll(domain.RunViewScrollLines), nil
	case keyScrollDwn:
		return m.scroll(-domain.RunViewScrollLines), nil
	case keyLive:
		return m.scrollToLive(), nil
	}
	return m, nil
}

// handleFilterKey reads the filter box. Every keystroke re-resolves the
// selection: a filter that hides the selected job moves the cursor rather than
// leaving a pane on screen the list no longer offers.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyInterrupt:
		return m.detach()
	case keyEscape:
		m.filtering, m.filter = false, ""
		return m.setSelection(m.resolveSelection())
	case keyEnter:
		m.filtering = false
		return m, nil
	case keyBackspace:
		if m.filter == "" {
			return m, nil
		}
		runes := []rune(m.filter)
		m.filter = string(runes[:len(runes)-1])
		return m.setSelection(m.resolveSelection())
	}

	if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
		return m, nil
	}
	m.filter += string(msg.Runes)
	return m.setSelection(m.resolveSelection())
}

// detach leaves the view. Nothing is stopped: the streams are dropped, and the
// jobs behind them keep running.
func (m Model) detach() (tea.Model, tea.Cmd) {
	m.panes.closeAll()
	return m, tea.Quit
}

func (m Model) move(delta int) (tea.Model, tea.Cmd) {
	visible := m.visible()
	if len(visible) == 0 {
		return m, nil
	}
	index := rules.ClampIndex(m.selectedIndex()+delta, len(visible))
	return m.setSelection(visible[index].Name)
}

// setSelection moves the cursor and releases the pane it leaves behind: only
// the selected job holds a subscription, so the one being left has to give its
// own up before the next is opened.
func (m Model) setSelection(name string) (Model, tea.Cmd) {
	if name != m.selected {
		m.panes.release(m.selected)
		m.selected = name
	}
	m.offset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.selectedIndex(),
		Total:   len(m.visible()),
		Visible: m.layout().SidebarRows,
		Offset:  m.offset,
	})
	return m.fillSelectedPane()
}

func (m Model) scroll(lines int) Model {
	entry, held := m.panes.entry(m.selected)
	if !held {
		return m
	}
	entry.pane.ScrollUp(lines)
	return m
}

func (m Model) scrollToLive() Model {
	entry, held := m.panes.entry(m.selected)
	if !held {
		return m
	}
	entry.pane.ScrollToLive()
	return m
}
