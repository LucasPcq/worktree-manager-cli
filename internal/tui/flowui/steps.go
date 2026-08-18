package flowui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// loadRequestMsg asks the message handler to load a step's content; loadDoneMsg
// carries it back. A step whose content needs I/O is loaded when it is entered,
// behind the wizard's loading spinner, so the render path never blocks.
type (
	loadRequestMsg struct {
		idx     int
		answers flow.Answers
	}
	loadDoneMsg struct {
		idx     int
		content flow.StepContent
		err     error
	}
)

// componentStep translates one flow step. conditional marks a step that decides on
// entry whether it applies at all.
func (p *plan) componentStep(step flow.Step, conditional bool) (components.Step, error) {
	if conditional {
		return p.choiceStep(step), nil
	}
	switch step.Kind {
	case flow.StepText:
		return p.textStep(step), nil
	case flow.StepSelect:
		return p.selectStep(step)
	case flow.StepBranchSelect:
		return p.branchStep(step)
	case flow.StepRecap:
		return p.recapStep(step), nil
	}
	return components.Step{}, unsupportedKindErr(step)
}

func (p *plan) textStep(step flow.Step) components.Step {
	return components.Step{
		Name: step.Label,
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       step.Title,
			Description: step.Description,
			Validate:    step.Validate,
		}),
		Summary: summaryFor(step),
	}
}

func (p *plan) selectStep(step flow.Step) (components.Step, error) {
	content, err := p.content(step, p.known())
	if err != nil {
		return components.Step{}, err
	}
	built := components.Step{
		Name:    step.Label,
		Model:   selectList(content),
		Summary: summaryFor(step),
	}
	if step.Build != nil {
		built.Build = func(prev []components.Step) any {
			return selectList(p.rebuild(step, prev))
		}
	}
	return built, nil
}

// branchStep renders a branch picker: every row carries its divergence from
// origin, and the whole list can be re-fetched in place.
func (p *plan) branchStep(step flow.Step) (components.Step, error) {
	p.candidates = step.Branches
	p.refresh = step.Refresh

	model := func(answers flow.Answers) (any, error) {
		content, err := p.content(step, answers)
		if err != nil {
			return nil, err
		}
		return components.NewSelectList(components.NewSelectListParams{
			Title:       content.Title,
			Description: content.Description,
			Items:       p.branchItems(step.Pinned),
		}), nil
	}

	initial, err := model(p.known())
	if err != nil {
		return components.Step{}, err
	}
	built := components.Step{
		Name:  step.Label,
		Model: initial,
		Build: func(prev []components.Step) any {
			rebuilt, buildErr := model(p.answersFrom(prev))
			if buildErr != nil {
				p.loadErr = buildErr
				return components.NewSelectList(components.NewSelectListParams{Title: step.Title})
			}
			return rebuilt
		},
		CanRefresh: step.Refresh != nil,
		Summary:    summaryFor(step),
	}
	if step.Refresh != nil {
		p.initCmd = branchrefresh.CmdFunc(step.Refresh)
		p.loadingText = domain.LoadingBranchesText
	}
	return built, nil
}

// branchItems renders the candidates, pinning the suggested default first when it
// is one of them.
func (p *plan) branchItems(pinned string) []components.SelectItem {
	found := ""
	for _, candidate := range p.candidates {
		if candidate.Name == pinned {
			found = pinned
			break
		}
	}
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   p.candidates,
		Pinned:       found,
		PinnedSuffix: domain.PinnedSuffixDefault,
	})
}

// choiceStep renders a step that decides on entry whether it applies, skipping
// itself with a reason when it does not. Every option merely advances: going back
// returns to the previous step, so a choice never cancels the operation.
func (p *plan) choiceStep(step flow.Step) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    step.Label,
		Summary: summaryFor(step),
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			answers := p.answersFrom(prev)
			if skip, reason := step.Skip(answers); skip {
				return false, reason, components.NewSelectListParams{}
			}
			content := p.rebuild(step, prev)
			return true, "", components.NewSelectListParams{
				Title:       content.Title,
				Description: content.Description,
				Items:       toItems(content.Options),
			}
		},
	})
}

// recapStep renders the final synthesis, with the constant cancel row appended as
// the single explicit cancellation point. A recap whose body needs I/O is loaded
// when it is entered instead of being built inline.
func (p *plan) recapStep(step flow.Step) components.Step {
	if step.Load != nil {
		return p.loadedRecapStep(step)
	}
	build := func(answers flow.Answers) any {
		content, err := p.content(step, answers)
		if err != nil {
			p.loadErr = err
			return placeholder(step)
		}
		return recapList(content)
	}
	return components.Step{
		Name:    step.Label,
		Model:   build(p.known()),
		Recap:   true,
		Summary: summaryFor(step),
		Build:   func(prev []components.Step) any { return build(p.answersFrom(prev)) },
	}
}

// loadedRecapStep shows an empty recap — where Enter is a no-op — until the loaded
// body replaces it, so a run is never confirmed before its consequences are visible.
func (p *plan) loadedRecapStep(step flow.Step) components.Step {
	idx := len(p.steps)
	if p.loads == nil {
		p.loads = map[int]flow.Step{}
	}
	p.loads[idx] = step

	return components.Step{
		Name:    step.Label,
		Model:   placeholder(step),
		Recap:   true,
		Summary: summaryFor(step),
		OnEnter: func(prev []components.Step) tea.Cmd {
			answers := p.answersFrom(prev)
			return func() tea.Msg { return loadRequestMsg{idx: idx, answers: answers} }
		},
	}
}

// handler routes the messages the translated steps rely on: the branch refresh and
// the asynchronous step loads.
func (p *plan) handler() components.WizardMsgHandler {
	var handlers []components.WizardMsgHandler
	if p.refresh != nil {
		handlers = append(handlers, branchrefresh.HandlerFunc(p.refresh, &p.candidates))
	}
	if len(p.loads) > 0 {
		handlers = append(handlers, p.loadHandler())
	}
	return combine(handlers...)
}

func (p *plan) loadHandler() components.WizardMsgHandler {
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		switch m := msg.(type) {
		case loadRequestMsg:
			step, ok := p.loads[m.idx]
			if !ok {
				return nil, false
			}
			// Clear whatever a previous answer left, so the spinner is not shown over a
			// stale body.
			w.UpdateStepModel(m.idx, func(any) any { return placeholder(step) })
			return tea.Batch(w.StartLoading(step.LoadingMessage), runLoad(m.idx, step, m.answers)), true
		case loadDoneMsg:
			step, ok := p.loads[m.idx]
			if !ok {
				return nil, false
			}
			content := m.content
			if m.err != nil {
				p.loadErr = m.err
				content = flow.StepContent{Title: step.Title, Description: m.err.Error()}
			}
			w.UpdateStepModel(m.idx, func(any) any { return recapList(content) })
			w.SetLoading(false)
			return nil, true
		}
		return nil, false
	}
}

// runLoad runs a step's Load off the UI goroutine so the spinner keeps animating.
func runLoad(idx int, step flow.Step, answers flow.Answers) tea.Cmd {
	return func() tea.Msg {
		content, err := step.Load(answers)
		return loadDoneMsg{idx: idx, content: content, err: err}
	}
}

// combine chains message handlers, stopping at the first that consumes the message.
func combine(handlers ...components.WizardMsgHandler) components.WizardMsgHandler {
	if len(handlers) == 0 {
		return nil
	}
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		for _, handle := range handlers {
			if cmd, handled := handle(w, msg); handled {
				return cmd, true
			}
		}
		return nil, false
	}
}

// content merges what a step declares statically with what it derives from the
// answers: a Build only has to return the parts that actually change.
func (p *plan) content(step flow.Step, answers flow.Answers) (flow.StepContent, error) {
	content := flow.StepContent{Title: step.Title, Description: step.Description, Options: step.Options}
	if step.Build == nil {
		return content, nil
	}
	built, err := step.Build(answers)
	if err != nil {
		return flow.StepContent{}, err
	}
	if built.Title != "" {
		content.Title = built.Title
	}
	if built.Description != "" {
		content.Description = built.Description
	}
	if len(built.Options) > 0 {
		content.Options = built.Options
	}
	return content, nil
}

// rebuild derives a step's content from the completed prior steps, falling back to
// what it declares statically when the derivation fails (the failure is surfaced
// once the wizard exits).
func (p *plan) rebuild(step flow.Step, prev []components.Step) flow.StepContent {
	content, err := p.content(step, p.answersFrom(prev))
	if err != nil {
		p.loadErr = err
		return flow.StepContent{Title: step.Title, Description: step.Description}
	}
	return content
}

func selectList(content flow.StepContent) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       toItems(content.Options),
	})
}

// recapList renders a recap body plus the constant cancel row.
func recapList(content flow.StepContent) components.SelectListModel {
	items := append(toItems(content.Options),
		components.SelectItem{Separator: true},
		components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
	)
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
	})
}

// placeholder is a step with nothing to choose: Enter is a no-op on it.
func placeholder(step flow.Step) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{Title: step.Title})
}

func toItems(options []flow.Option) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(options))
	for _, option := range options {
		items = append(items, components.SelectItem{
			Label:     option.Label,
			Value:     option.Value,
			Separator: option.Separator,
			Danger:    option.Danger,
		})
	}
	return items
}

// summaryFor labels a completed step: the flow's own wording when it has one, else
// the answer as it was given.
func summaryFor(step flow.Step) func(any) string {
	if step.Summarize == nil {
		if step.Kind == flow.StepText {
			return components.TextSummary
		}
		return components.SelectSummary
	}
	return func(model any) string { return step.Summarize(answerOf(step.Kind, model)) }
}
