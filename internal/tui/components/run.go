package components

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// standaloneModel wraps a child model for use as a standalone tea.Program.
// It adds a title, description, and help bar around the child's View.
type standaloneModel struct {
	child   any
	title   string
	desc    string
	width   int
	done    bool
	aborted bool
}

// RunStandaloneSelect runs a SelectList as a standalone tea.Program.
// Returns the selected value or ErrAborted.
func RunStandaloneSelect(sl SelectListModel) (string, error) {
	m := standaloneModel{
		child: sl,
		title: sl.title,
		desc:  sl.desc,
		width: 80,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("select: %w", err)
	}

	final, ok := finalModel.(standaloneModel)
	if !ok || final.aborted {
		return "", ErrAborted
	}

	child, ok := final.child.(SelectListModel)
	if !ok {
		return "", ErrAborted
	}

	return child.Value(), nil
}

// RunStandaloneConfirm runs a ConfirmModel as a standalone tea.Program.
// Returns true for Yes, false for No, or ErrAborted on Esc.
func RunStandaloneConfirm(cm ConfirmModel) (bool, error) {
	m := standaloneModel{
		child: cm,
		title: cm.title,
		desc:  cm.desc,
		width: 80,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("confirm: %w", err)
	}

	final, ok := finalModel.(standaloneModel)
	if !ok || final.aborted {
		return false, ErrAborted
	}

	child, ok := final.child.(ConfirmModel)
	if !ok {
		return false, ErrAborted
	}

	return child.Confirmed(), nil
}

// ErrAborted is returned when the user presses Esc in a standalone component.
var ErrAborted = fmt.Errorf("user aborted")

func (m standaloneModel) Init() tea.Cmd {
	switch child := m.child.(type) {
	case SelectListModel:
		return child.Init()
	case ConfirmModel:
		return child.Init()
	}
	return nil
}

func (m standaloneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsMsg.Width
		switch child := m.child.(type) {
		case SelectListModel:
			child.width = m.width
			child.height = max(1, wsMsg.Height-6)
			m.child = child
		case ConfirmModel:
			child.width = m.width
			m.child = child
		}
		return m, nil
	}

	switch child := m.child.(type) {
	case SelectListModel:
		updated, cmd := child.Update(msg)
		m.child = updated
		if updated.Chosen() {
			m.done = true
			return m, tea.Quit
		}
		if updated.Aborted() {
			m.aborted = true
			return m, tea.Quit
		}
		return m, cmd
	case ConfirmModel:
		updated, cmd := child.Update(msg)
		m.child = updated
		if updated.Done() {
			m.done = true
			return m, tea.Quit
		}
		if updated.Aborted() {
			m.aborted = true
			return m, tea.Quit
		}
		return m, cmd
	}

	return m, nil
}

func (m standaloneModel) View() string {
	if m.done || m.aborted {
		return ""
	}

	out := "\n"

	if m.title != "" {
		out += styles.BreadcrumbActive.Render(styles.Indent + m.title)
		out += "\n\n"
	}

	if m.desc != "" {
		for _, line := range strings.Split(m.desc, "\n") {
			out += styles.Muted.Render(styles.Indent + line)
			out += "\n"
		}
		out += "\n"
	}

	switch child := m.child.(type) {
	case SelectListModel:
		out += child.View()
	case ConfirmModel:
		out += child.View()
	}

	out += "\n\n"

	help := "  enter confirm"
	if _, ok := m.child.(SelectListModel); ok {
		help += " • / filter"
	}
	help += " • esc cancel"
	out += styles.HelpBar.Render(help)
	out += "\n"

	return out
}
