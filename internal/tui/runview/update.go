package runview

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focused {
		return m.handleFocusKey(msg)
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	// A refusal answers one keystroke; the next one is a fresh question.
	m.notice = ""

	// Any key the reader presses is them taking the cursor back from the run.
	m.following = false

	switch msg.String() {
	case domain.RunViewFocusKey:
		return m.focus()
	case keyEscape:
		return m.dismiss()
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

// handleFocusKey hands the keyboard to the job: every key it can encode is
// written to the job's stdin, Ctrl+C included — interrupting the child is the
// whole point of focus. The one key it keeps is the one that gives the keyboard
// back, or there would be no way out.
func (m Model) handleFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == domain.RunViewFocusExitKey {
		m.focused = false
		return m, nil
	}

	stream := m.panes.stream(m.selected)
	if stream == nil {
		m.focused = false
		return m, nil
	}
	encoded := rules.EncodeKeyStroke(strokeOf(msg))
	if len(encoded) == 0 {
		return m, nil
	}
	return m, writeCmd(writeParams{Job: m.selected, Stream: stream, Bytes: encoded})
}

// focus is refused for a pane the job is not behind: a log file has no stdin,
// and a keystroke silently going nowhere reads as a frozen terminal.
func (m Model) focus() (tea.Model, tea.Cmd) {
	if m.panes.stream(m.selected) == nil {
		m.notice = fmt.Sprintf(domain.RunViewNotAttachableFmt, m.selected)
		return m, nil
	}
	m.focused, m.notice = true, ""
	return m.scrollToLive(), nil
}

func strokeOf(msg tea.KeyMsg) domain.KeyStroke {
	if msg.Type == tea.KeyRunes {
		return domain.KeyStroke{Runes: msg.Runes, Alt: msg.Alt}
	}
	return domain.KeyStroke{Name: msg.Type.String(), Alt: msg.Alt}
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

// detach leaves the view. Nothing is stopped: cancelling ends the reporting of
// a run in progress, not the run, and the jobs behind the dropped streams keep
// running.
func (m Model) detach() (tea.Model, tea.Cmd) {
	m.cancel()
	m.panes.closeAll()
	return m, tea.Quit
}

// dismiss takes the abort report off the frame and gives its rows back to the
// panes under it. With no report on screen the key does nothing: pressing esc
// before the run fails would otherwise silence a report that is not written
// yet, and the reader would never learn why the profile stopped.
func (m Model) dismiss() (tea.Model, tea.Cmd) {
	if len(m.report()) == 0 {
		return m, nil
	}
	m.dismissed = true
	model, cmd := m.applySize()
	return model, cmd
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
		m.selected, m.focused = name, false
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
