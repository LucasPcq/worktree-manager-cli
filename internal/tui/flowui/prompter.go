// Package flowui runs a flow.Session as a wtm wizard: it translates the steps a
// flow declares into Bubbletea models and reads the answers back out. It is the
// only place that knows both vocabularies — a flow never sees a model, and the
// wizard never sees a service.
package flowui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Params configures the prompter.
type Params struct {
	// Stderr renders on stderr instead of stdout, for a command whose stdout is
	// consumed by the shell wrapper.
	Stderr bool
}

// Prompter answers a flow's questions interactively.
type Prompter struct {
	params Params
}

// New returns the interactive prompter.
func New(params Params) Prompter { return Prompter{params: params} }

// Interactive reports that questions can be asked.
func (Prompter) Interactive() bool { return true }

// Confirm shows a single confirmation outside any wizard.
func (Prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	return components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:       params.Title,
		Description: params.Description,
		Warning:     params.Warning,
		DefaultYes:  params.DefaultYes,
	}))
}

// Ask renders the session as one wizard — a single breadcrumb, back navigation
// between steps, and one recap — and returns every answer. A step the session
// already has a preset for is not shown, but its value is still returned.
func (p Prompter) Ask(session flow.Session) (flow.Answers, error) {
	if p.params.Stderr {
		// The wizard may be reached through a shell wrapper that captures stdout, so
		// force color detection against stderr (the TTY).
		styles.UseRendererOn(os.Stderr)
	}

	plan, err := build(session)
	if err != nil {
		return flow.Answers{}, err
	}
	// Every value was already known: there is nothing to ask.
	if len(plan.steps) == 0 {
		return plan.known(), nil
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:       plan.steps,
		Stderr:      p.params.Stderr,
		ErrLabel:    session.ErrLabel,
		InitCmd:     plan.initCmd,
		OnMsg:       plan.handler(),
		Loading:     plan.initCmd != nil,
		LoadingText: plan.loadingText,
	})
	if err != nil {
		return flow.Answers{}, err
	}
	if plan.loadErr != nil {
		return flow.Answers{}, plan.loadErr
	}
	return plan.read(final)
}

// unsupportedKindErr refuses a step kind this host cannot render. A surface must
// say so rather than guess: the kinds extract and sync need (multi-select,
// standalone confirm) arrive with them.
func unsupportedKindErr(step flow.Step) error {
	return fmt.Errorf("flowui: step %q has no renderer for kind %d", step.Key, step.Kind)
}

// binding ties one wizard step back to the flow step it renders.
type binding struct {
	key  string
	kind flow.StepKind
}

// plan is a session translated into wizard steps, plus what is needed to read the
// answers back.
type plan struct {
	steps    []components.Step
	bindings []binding
	presets  flow.Answers
	// skips are the steps resolved as irrelevant before the wizard started (a
	// conditional step cannot be the wizard's first one).
	skips map[string]string
	// candidates backs every branch step, so a refresh replaces the list once and
	// each step's rebuild picks it up.
	candidates  []domain.BranchCandidate
	refresh     func() []domain.BranchCandidate
	initCmd     tea.Cmd
	loadingText string
	// loads are the steps whose content is loaded asynchronously, by wizard index.
	loads map[int]flow.Step
	// loadErr keeps a failure of a step's Load, surfaced once the wizard exits.
	loadErr error
}

// build translates the session, constructing each step's initial model up front so
// a step that cannot even be built refuses the run before anything is displayed.
func build(session flow.Session) (*plan, error) {
	p := &plan{presets: session.Presets, skips: map[string]string{}}

	for _, step := range session.Steps {
		if _, preset := session.Presets.Get(step.Key); preset {
			continue
		}

		// A conditional step cannot be the wizard's first step: the wizard neither
		// builds nor auto-skips step 0. Resolve its condition here instead, against
		// what is already known, and include it only if it applies.
		conditional := step.Skip != nil
		if conditional && len(p.steps) == 0 {
			if skip, reason := step.Skip(p.known()); skip {
				p.skips[step.Key] = reason
				continue
			}
			conditional = false
		}

		built, err := p.componentStep(step, conditional)
		if err != nil {
			return nil, err
		}
		p.steps = append(p.steps, built)
		p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind})
	}
	return p, nil
}

// known returns the answers available before any step ran: the presets plus the
// steps already resolved as irrelevant.
func (p *plan) known() flow.Answers {
	answers := p.presets
	for key, reason := range p.skips {
		answers = answers.With(key, flow.Answer{Skipped: true, SkipReason: reason})
	}
	return answers
}

// answersFrom derives the answers so far from the wizard's completed steps, so a
// step's Build, Skip or Load sees exactly what the user has answered.
func (p *plan) answersFrom(prev []components.Step) flow.Answers {
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(prev) {
			break
		}
		answers = answers.With(b.key, answerOf(b.kind, prev[i].Model))
	}
	return answers
}

// read collects the final answers: the presets, the skipped steps, and what each
// wizard step holds. A cancelled recap is an abort.
func (p *plan) read(final components.WizardModel) (flow.Answers, error) {
	steps := final.Steps()
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(steps) {
			break
		}
		answer := answerOf(b.kind, steps[i].Model)
		if answer.Value == domain.WizardCancelValue {
			return flow.Answers{}, domain.ErrUserAborted
		}
		if final.Skipped(i) {
			answer = flow.Answer{Skipped: true, SkipReason: skipReasonOf(steps[i])}
		}
		answers = answers.With(b.key, answer)
	}
	return answers, nil
}

func skipReasonOf(step components.Step) string {
	if step.SkipReason == nil {
		return ""
	}
	return step.SkipReason()
}

// answerOf reads a step's model. It is the single place where the wizard's
// model-per-kind switch is crossed; everything above it speaks flow.Answer.
func answerOf(kind flow.StepKind, model any) flow.Answer {
	switch kind {
	case flow.StepText:
		if text, ok := model.(components.TextInputModel); ok {
			return flow.Answer{Value: text.Value(), Asked: true}
		}
	case flow.StepSelect, flow.StepBranchSelect, flow.StepRecap:
		if list, ok := model.(components.SelectListModel); ok {
			return flow.Answer{Value: list.Value(), Asked: true}
		}
	}
	return flow.Answer{}
}
