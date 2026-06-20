package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// Step defines one step in a wizard.
// Model must be a SelectListModel, TextInputModel, ConfirmModel, MultiSelectModel,
// or ReorderListModel.
type Step struct {
	Name  string
	Model any
	// Build, when set, rebuilds Model from the already-completed prior steps each
	// time the wizard enters this step. Use it for steps whose contents depend on
	// an earlier selection.
	Build   func(prev []Step) any
	Summary func(model any) string
	// AutoSkip, when set and returning true on entry, skips this step
	// automatically and advances. Use it for steps that are irrelevant given an
	// earlier answer (e.g. a sub-step whose section gate was set to "skip").
	// Skipped steps are hidden from the breadcrumb and reported via Skipped(i).
	AutoSkip func(w WizardModel) bool
	// Callout renders this step's description as an emphasized intro callout
	// (bold title + accent bar) instead of plain muted text. Use it for section
	// gates that explain what a section does.
	Callout bool
	// CalloutNote is an optional secondary line shown in the callout (e.g.
	// "Detected: …"). Only used when Callout is true.
	CalloutNote string
}

// WizardModel manages a multi-step form with breadcrumb and back navigation.
type WizardModel struct {
	steps   []Step
	skipped []bool
	current int
	width   int
	height  int
	done    bool
	aborted bool
}

// NewWizard creates a wizard with the given steps.
func NewWizard(steps []Step) WizardModel {
	return WizardModel{
		steps:   steps,
		skipped: make([]bool, len(steps)),
		width:   80,
	}
}

// NewWizardAtStep creates a wizard positioned on the given step, with all prior
// steps treated as already completed (their summaries show in the breadcrumb and
// back navigation reaches them). Used to re-enter a flow without redoing earlier
// answers. An out-of-range start falls back to the first step.
func NewWizardAtStep(steps []Step, start int) WizardModel {
	m := NewWizard(steps)
	if start > 0 && start < len(steps) {
		m.current = start
	}
	return m
}

// Done returns true when all steps have been completed.
func (m WizardModel) Done() bool { return m.done }

// Aborted returns true when the user aborted at the first step.
func (m WizardModel) Aborted() bool { return m.aborted }

// Steps returns the wizard steps for value extraction.
func (m WizardModel) Steps() []Step { return m.steps }

// Skipped reports whether the step at index i was skipped by the user.
func (m WizardModel) Skipped(i int) bool {
	if i < 0 || i >= len(m.skipped) {
		return false
	}
	return m.skipped[i]
}

// Init initializes the current step's model (the first step unless the wizard
// was created with NewWizardAtStep).
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	m.propagateSize(m.current)
	return m.initStep(m.current)
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
	case ReorderListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case HookListModel:
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
		if m.skipped[i] {
			continue
		}
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
		if step.Callout {
			b.WriteString(styles.RenderIntro(styles.IntroParams{
				Width: m.width,
				Body:  desc,
				Note:  step.CalloutNote,
			}))
		} else {
			b.WriteString(styles.Muted.Render(indentLines(desc)))
		}
		b.WriteString("\n\n")
	}
	b.WriteString(m.viewStep(m.current))

	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar())
	b.WriteString("\n")

	return b.String()
}

func (m WizardModel) renderBreadcrumb() string {
	counter := styles.Breadcrumb.Render(fmt.Sprintf("  Step %d/%d", m.visiblePosition(), m.visibleCount()))
	sep := styles.Breadcrumb.Render(" • ")
	name := styles.BreadcrumbActive.Render(m.steps[m.current].Name)
	return counter + sep + name
}

// visibleCount returns the number of steps not skipped (the effective length).
func (m WizardModel) visibleCount() int {
	n := 0
	for i := range m.steps {
		if !m.skipped[i] {
			n++
		}
	}
	return n
}

// visiblePosition returns the 1-based index of the current step among visible steps.
func (m WizardModel) visiblePosition() int {
	n := 0
	for i := 0; i <= m.current && i < len(m.steps); i++ {
		if !m.skipped[i] {
			n++
		}
	}
	return n
}

func (m WizardModel) renderHelpBar() string {
	if hl, ok := m.steps[m.current].Model.(HookListModel); ok {
		return styles.HelpBar.Render(hl.helpHint())
	}

	help := "  enter confirm"
	switch m.steps[m.current].Model.(type) {
	case SelectListModel:
		help += " • / filter"
	case MultiSelectModel:
		help += " • space toggle"
	case ReorderListModel:
		help += " • shift+↑/↓ move"
	}
	if m.visiblePosition() > 1 {
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
	case ReorderListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case HookListModel:
		child.width = m.width
		child.height = h
		child.cmdInput.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
		child.cwdInput.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
		m.steps[stepIdx].Model = child
	}
}

func (m WizardModel) advance() (tea.Model, tea.Cmd) {
	m.current++
	for m.current < len(m.steps) {
		m.buildStep(m.current)
		if m.steps[m.current].AutoSkip != nil && m.steps[m.current].AutoSkip(m) {
			m.skipped[m.current] = true
			m.current++
			continue
		}
		break
	}
	if m.current >= len(m.steps) {
		m.done = true
		return m, tea.Quit
	}
	m.propagateSize(m.current)
	return m, m.initStep(m.current)
}

// buildStep rebuilds a step's model from prior steps when a Build hook is set.
func (m *WizardModel) buildStep(stepIdx int) {
	if stepIdx <= 0 || stepIdx >= len(m.steps) {
		return
	}
	step := &m.steps[stepIdx]
	if step.Build == nil {
		return
	}
	step.Model = step.Build(m.steps[:stepIdx])
}

func (m WizardModel) goBack() (tea.Model, tea.Cmd) {
	// Land on the nearest previous visible step, hopping over auto-skipped ones.
	prev := m.current - 1
	for prev >= 0 && m.skipped[prev] {
		prev--
	}
	if prev < 0 {
		m.aborted = true
		return m, tea.Quit
	}
	m.resetStep(m.current)
	// Clear skip flags for the steps we hop back over so re-advancing
	// re-evaluates AutoSkip against the (possibly changed) gate answer.
	for i := prev; i < m.current; i++ {
		m.skipped[i] = false
	}
	m.current = prev
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
	case ReorderListModel:
		return child.Init()
	case HookListModel:
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
	case ReorderListModel:
		return child.View()
	case HookListModel:
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
	case ReorderListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case HookListModel:
		child.done = false
		child.aborted = false
		child.editing = false
		m.steps[stepIdx].Model = child
	}
}

// indentLines prefixes every line with the standard indent so multi-line
// descriptions stay aligned (lipgloss only pads the first line otherwise).
func indentLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = styles.Indent + line
	}
	return strings.Join(lines, "\n")
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
	case ReorderListModel:
		return child.desc
	case HookListModel:
		return child.desc
	}
	return ""
}
