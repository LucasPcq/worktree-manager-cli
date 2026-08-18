package dashboard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/styles"
)

type formRowKind int

const (
	formText formRowKind = iota
	formChoice
	formBlocker
	formButton
)

// formRow is one line of a form modal. It describes what to draw and what
// activating it answers; whether it is ticked lives in the modal, so a rebuild of
// the rows never drops what the user already lifted.
type formRow struct {
	kind    formRowKind
	stepKey string
	value   string
	label   string
	danger  bool
	confirm bool
}

func (r formRow) interactive() bool { return r.kind != formText }

type formReadyMsg struct {
	generation int
	rows       []formRow
	// answers is what the form on screen amounts to, defaults included: submitting
	// it is confirming the whole thing.
	answers flow.Answers
	err     error
}

func buildFormCmd(session flow.Session, chosen flow.Answers, generation int) tea.Cmd {
	return func() tea.Msg {
		rows, answers, err := buildFormRows(session, chosen)
		return formReadyMsg{generation: generation, rows: rows, answers: answers, err: err}
	}
}

// buildFormRows lays a whole session out at once. Each step is rendered against
// the answers of the ones before it, so a choice the user changes is reflected by
// what the later sections say.
func buildFormRows(session flow.Session, chosen flow.Answers) ([]formRow, flow.Answers, error) {
	answers := session.Presets
	var rows []formRow

	for _, step := range session.Steps {
		if _, preset := session.Presets.Get(step.Key); preset {
			continue
		}
		if step.Skip != nil {
			if skip, _ := step.Skip(answers); skip {
				continue
			}
		}

		content, err := stepContent(step, answers)
		if err != nil {
			return nil, flow.Answers{}, err
		}
		section, answer := formSection(formSectionParams{
			Step:    step,
			Content: content,
			Chosen:  chosen,
			Answers: answers,
		})
		rows = append(rows, section...)
		answers = answers.With(step.Key, answer)
	}
	return rows, answers, nil
}

type formSectionParams struct {
	Step    flow.Step
	Content flow.StepContent
	Chosen  flow.Answers
	Answers flow.Answers
}

// formSection renders one step and reports the answer the later steps must read.
func formSection(params formSectionParams) ([]formRow, flow.Answer) {
	rows := heading(params.Content)

	if params.Step.Kind == flow.StepRecap {
		return append(rows, blockerRows(params.Content)...), flow.Answer{
			Value: confirmValue(params.Content),
			Asked: true,
		}
	}

	value := selectedValue(params)
	for _, option := range params.Content.Options {
		if option.Separator {
			continue
		}
		rows = append(rows, formRow{
			kind:    formChoice,
			stepKey: params.Step.Key,
			value:   option.Value,
			label:   option.Label,
			danger:  option.Danger,
		})
	}
	return append(rows, formRow{kind: formText}), flow.Answer{Value: value, Asked: true}
}

// selectedValue is what the step is currently answered with: the user's own
// choice, else the answer the flow resolves to when nobody is asked — never one
// this surface picked for itself.
func selectedValue(params formSectionParams) string {
	if answer, ok := params.Chosen.Get(params.Step.Key); ok {
		return answer.Value
	}
	if params.Step.Resolve != nil {
		if answer, err := params.Step.Resolve(params.Answers); err == nil {
			return answer.Value
		}
	}
	return ""
}

func heading(content flow.StepContent) []formRow {
	var rows []formRow
	if content.Title != "" {
		rows = append(rows, formRow{kind: formText, label: content.Title})
	}
	for _, line := range strings.Split(content.Description, "\n") {
		rows = append(rows, formRow{kind: formText, label: line})
	}
	return append(rows, formRow{kind: formText})
}

// blockerRows turns each refusal into its own acknowledgement. Nothing is ticked
// for the user, and the button below them stays inert until they all are.
func blockerRows(content flow.StepContent) []formRow {
	var rows []formRow
	if len(content.Blockers) > 0 {
		rows = append(rows, formRow{kind: formText, label: domain.DashboardBlockersTitle})
	}
	for _, blocker := range content.Blockers {
		rows = append(rows, formRow{
			kind:  formBlocker,
			value: blocker.Key,
			label: blocker.Label,
		})
	}
	if len(rows) > 0 {
		rows = append(rows, formRow{kind: formText})
	}
	return append(rows,
		formRow{
			kind:    formButton,
			confirm: true,
			label:   confirmLabel(content),
			danger:  len(content.Blockers) > 0,
		},
		formRow{kind: formButton, label: domain.WizardCancelLabel},
	)
}

// confirmValue is the option the blockers stand in the way of: with something to
// lift, confirming means the dangerous one — there is no other way through.
func confirmValue(content flow.StepContent) string {
	if len(content.Blockers) > 0 {
		if option, ok := dangerOption(content); ok {
			return option.Value
		}
	}
	for _, option := range content.Options {
		if !option.Separator {
			return option.Value
		}
	}
	return ""
}

func confirmLabel(content flow.StepContent) string {
	if len(content.Blockers) > 0 {
		if option, ok := dangerOption(content); ok {
			return option.Label
		}
	}
	for _, option := range content.Options {
		if !option.Separator {
			return option.Label
		}
	}
	return domain.DashboardConfirmLabel
}

func dangerOption(content flow.StepContent) (flow.Option, bool) {
	for _, option := range content.Options {
		if option.Danger {
			return option, true
		}
	}
	return flow.Option{}, false
}

func (mo modal) applyForm(msg formReadyMsg) (modal, tea.Cmd) {
	if msg.generation != mo.generation {
		return mo, nil
	}
	if msg.err != nil {
		return mo.fail(msg.err)
	}
	mo.loading = ""
	mo.rows, mo.answers = msg.rows, msg.answers
	mo.focus = clampFocus(mo.rows, mo.focus)
	return mo, nil
}

func clampFocus(rows []formRow, focus int) int {
	for index := focus; index < len(rows); index++ {
		if rows[index].interactive() {
			return index
		}
	}
	for index := min(focus, len(rows)) - 1; index >= 0; index-- {
		if rows[index].interactive() {
			return index
		}
	}
	return 0
}

func (mo modal) updateForm(msg tea.KeyMsg) (modal, tea.Cmd) {
	switch msg.String() {
	case keyUp, keyVimUp:
		return mo.moveFocus(-1), nil
	case keyDown, keyVimDown:
		return mo.moveFocus(1), nil
	case keySpace, keyEnter:
		return mo.activate(mo.focus)
	case keyEscape:
		return mo.cancel()
	}
	return mo, nil
}

func (mo modal) moveFocus(delta int) modal {
	for index := mo.focus + delta; index >= 0 && index < len(mo.rows); index += delta {
		if mo.rows[index].interactive() {
			mo.focus = index
			return mo
		}
	}
	return mo
}

func (mo modal) activate(index int) (modal, tea.Cmd) {
	if index < 0 || index >= len(mo.rows) {
		return mo, nil
	}
	row := mo.rows[index]
	mo.focus = index

	switch row.kind {
	case formChoice:
		mo.chosen = mo.chosen.With(row.stepKey, flow.Answer{Value: row.value, Asked: true})
		mo.generation++
		return mo, buildFormCmd(mo.session, mo.chosen, mo.generation)
	case formBlocker:
		return mo.lift(row.value), nil
	case formButton:
		if !row.confirm {
			return mo.cancel()
		}
		if !mo.blockersLifted() {
			return mo, nil
		}
		return mo.submit(mo.answers)
	}
	return mo, nil
}

// lift copies the acknowledgements: the model is passed by value, and a shared
// map would tick a box in frames the user has already left behind.
func (mo modal) lift(key string) modal {
	lifted := make(map[string]bool, len(mo.lifted)+1)
	for k, v := range mo.lifted {
		lifted[k] = v
	}
	lifted[key] = !lifted[key]
	mo.lifted = lifted
	return mo
}

func (mo modal) blockersLifted() bool {
	for _, row := range mo.rows {
		if row.kind == formBlocker && !mo.lifted[row.value] {
			return false
		}
	}
	return true
}

func (mo modal) formBody(zones marker) []string {
	lines := make([]string, 0, len(mo.rows))
	for index, row := range mo.rows {
		line := mo.renderRow(index, row)
		if row.interactive() {
			line = zones.Mark(modalRowZone(index), line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (mo modal) renderRow(index int, row formRow) string {
	width := mo.bodyWidth()
	switch row.kind {
	case formText:
		return truncate(row.label, width)
	case formChoice:
		return mo.decorate(index, glyphChoice(mo.answers.Value(row.stepKey) == row.value)+" "+row.label, row)
	case formBlocker:
		return mo.decorate(index, glyphCheck(mo.lifted[row.value])+" "+row.label, row)
	}

	label := "[ " + row.label + " ]"
	if row.confirm && !mo.blockersLifted() {
		return styles.DashboardDisabled.Render(truncate(label+domain.DashboardBlockedSuffix, width))
	}
	return mo.decorate(index, label, row)
}

func (mo modal) decorate(index int, text string, row formRow) string {
	text = truncate(text, mo.bodyWidth())
	if index == mo.focus {
		return styles.DashboardRowFocused.Render(text)
	}
	if row.danger {
		return styles.DashboardDanger.Render(text)
	}
	return styles.DashboardRow.Render(text)
}

func glyphChoice(selected bool) string {
	if selected {
		return domain.DashboardGlyphChoiceOn
	}
	return domain.DashboardGlyphChoiceOff
}

func glyphCheck(checked bool) string {
	if checked {
		return domain.DashboardGlyphCheckOn
	}
	return domain.DashboardGlyphCheckOff
}
