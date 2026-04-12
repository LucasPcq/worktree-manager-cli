package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// Step defines one step in a wizard.
// Model must be a SelectListModel, TextInputModel, ConfirmModel, or MultiSelectModel.
type Step struct {
	Name    string
	Model   any
	Summary func(model any) string
}

// WizardModel manages a multi-step form with breadcrumb and back navigation.
type WizardModel struct {
	steps   []Step
	current int
	width   int
	height  int
	done    bool
	aborted bool
}

// NewWizard creates a wizard with the given steps.
func NewWizard(steps []Step) WizardModel {
	return WizardModel{
		steps: steps,
		width: 80,
	}
}

// Done returns true when all steps have been completed.
func (m WizardModel) Done() bool { return m.done }

// Aborted returns true when the user aborted at the first step.
func (m WizardModel) Aborted() bool { return m.aborted }

// Steps returns the wizard steps for value extraction.
func (m WizardModel) Steps() []Step { return m.steps }

// Init initializes the first step's model.
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	m.propagateSize(0)
	return m.initStep(0)
}

// Update delegates to the current step and manages transitions.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsMsg.Width
		m.height = wsMsg.Height
		m.propagateSize(m.current)
		return m, nil
	}

	if m.current >= len(m.steps) {
		return m, nil
	}

	step := &m.steps[m.current]

	advanced, back, cmd := m.updateStep(step, msg)
	if advanced {
		return m.advance()
	}
	if back {
		return m.goBack()
	}

	return m, cmd
}

func (m WizardModel) updateStep(step *Step, msg tea.Msg) (advanced bool, back bool, cmd tea.Cmd) {
	switch child := step.Model.(type) {
	case SelectListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Chosen(), updated.Aborted(), c
	case TextInputModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case ConfirmModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case MultiSelectModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	}
	return false, false, nil
}

// View renders the breadcrumb, completed step summaries, and the current step.
func (m WizardModel) View() string {
	if len(m.steps) == 0 || m.done || m.aborted {
		return ""
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.renderBreadcrumb())
	b.WriteString("\n\n")

	for i := 0; i < m.current; i++ {
		summary := ""
		if m.steps[i].Summary != nil {
			summary = m.steps[i].Summary(m.steps[i].Model)
		}
		line := fmt.Sprintf("  ✓ %s: %s", m.steps[i].Name, summary)
		b.WriteString(styles.SummaryLine.Render(line))
		b.WriteString("\n")
	}

	if m.current > 0 {
		b.WriteString("\n")
	}

	step := m.steps[m.current]
	if desc := m.stepDescription(step); desc != "" {
		b.WriteString(styles.Muted.Render("  " + desc))
		b.WriteString("\n\n")
	}
	b.WriteString(m.viewStep(m.current))

	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar())
	b.WriteString("\n")

	return b.String()
}

func (m WizardModel) renderBreadcrumb() string {
	counter := styles.Breadcrumb.Render(fmt.Sprintf("  Step %d/%d", m.current+1, len(m.steps)))
	sep := styles.Breadcrumb.Render(" • ")
	name := styles.BreadcrumbActive.Render(m.steps[m.current].Name)
	return counter + sep + name
}

func (m WizardModel) renderHelpBar() string {
	help := "  enter confirm"
	switch m.steps[m.current].Model.(type) {
	case SelectListModel:
		help += " • / filter"
	case MultiSelectModel:
		help += " • space toggle"
	}
	if m.current > 0 {
		help += " • esc back"
	} else {
		help += " • esc cancel"
	}
	return styles.HelpBar.Render(help)
}

func (m *WizardModel) propagateSize(stepIdx int) {
	if stepIdx >= len(m.steps) {
		return
	}
	h := max(1, m.height-10)
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case TextInputModel:
		child.width = m.width
		child.input.Width = max(10, m.width-4)
		m.steps[stepIdx].Model = child
	case ConfirmModel:
		child.width = m.width
		m.steps[stepIdx].Model = child
	case MultiSelectModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	}
}

func (m WizardModel) advance() (tea.Model, tea.Cmd) {
	m.current++
	if m.current >= len(m.steps) {
		m.done = true
		return m, tea.Quit
	}
	m.propagateSize(m.current)
	return m, m.initStep(m.current)
}

func (m WizardModel) goBack() (tea.Model, tea.Cmd) {
	if m.current == 0 {
		m.aborted = true
		return m, tea.Quit
	}
	m.resetStep(m.current)
	m.current--
	m.resetStep(m.current)
	m.propagateSize(m.current)
	return m, m.initStep(m.current)
}

func (m WizardModel) initStep(stepIdx int) tea.Cmd {
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		return child.Init()
	case TextInputModel:
		return child.Init()
	case ConfirmModel:
		return child.Init()
	case MultiSelectModel:
		return child.Init()
	}
	return nil
}

func (m WizardModel) viewStep(stepIdx int) string {
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		return child.View()
	case TextInputModel:
		return child.View()
	case ConfirmModel:
		return child.View()
	case MultiSelectModel:
		return child.View()
	}
	return ""
}

func (m *WizardModel) resetStep(stepIdx int) {
	if stepIdx >= len(m.steps) {
		return
	}
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		child.chosen = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case TextInputModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case ConfirmModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case MultiSelectModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	}
}

func (m WizardModel) stepDescription(step Step) string {
	switch child := step.Model.(type) {
	case SelectListModel:
		return child.desc
	case TextInputModel:
		return child.desc
	case ConfirmModel:
		return child.desc
	case MultiSelectModel:
		return child.desc
	}
	return ""
}
