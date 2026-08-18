package flowui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// These tests cover the translation from a flow's steps to wizard steps and back:
// which questions are shown, how a preset removes one, how a conditional step that
// would land first is resolved up front, and how the answers are read out again.

func textStep(key string) flow.Step {
	return flow.Step{Kind: flow.StepText, Key: key, Label: "Text " + key, Title: "Text"}
}

func selectStep(key string, values ...string) flow.Step {
	options := make([]flow.Option, 0, len(values))
	for _, value := range values {
		options = append(options, flow.Option{Label: value, Value: value})
	}
	return flow.Step{Kind: flow.StepSelect, Key: key, Label: "Select " + key, Options: options}
}

func recapStep(key string) flow.Step {
	return flow.Step{
		Kind: flow.StepRecap, Key: key, Label: "Recap",
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: "review",
				Options:     []flow.Option{{Label: "Yes, do it", Value: "go"}},
			}, nil
		},
	}
}

func names(steps []components.Step) []string {
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.Name)
	}
	return out
}

// TestBuildSkipsPresetSteps: a value the request already carries is not asked, but
// it is still returned — that is what keeps a flag from erasing a recap line.
func TestBuildSkipsPresetSteps(t *testing.T) {
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"a": "given"}),
		Steps:   []flow.Step{textStep("a"), selectStep("b", "one"), recapStep("r")},
	}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(plan.steps); strings.Join(got, ",") != "Select b,Recap" {
		t.Errorf("steps = %v, want the preset one dropped", got)
	}
	if got := plan.known().Value("a"); got != "given" {
		t.Errorf("preset value = %q, want it available to every step", got)
	}
}

// TestBuildResolvesAConditionalFirstStepUpFront: the wizard neither builds nor
// auto-skips its first step, so a conditional step that would land there is decided
// before the wizard starts — included when it applies, dropped when it does not.
func TestBuildResolvesAConditionalFirstStepUpFront(t *testing.T) {
	conditional := func(skip bool) flow.Step {
		return flow.Step{
			Kind: flow.StepSelect, Key: "c", Label: "Conditional",
			Skip: func(flow.Answers) (bool, string) { return skip, "nothing to reconcile" },
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Options: []flow.Option{{Label: "do", Value: "do"}}}, nil
			},
		}
	}

	dropped, err := build(flow.Session{Steps: []flow.Step{conditional(true), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(dropped.steps); strings.Join(got, ",") != "Recap" {
		t.Errorf("steps = %v, want the irrelevant step dropped", got)
	}
	answer, _ := dropped.known().Get("c")
	if !answer.Skipped || answer.SkipReason != "nothing to reconcile" {
		t.Errorf("answer = %+v, want it skipped with its reason", answer)
	}

	kept, err := build(flow.Session{Steps: []flow.Step{conditional(false), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(kept.steps); strings.Join(got, ",") != "Conditional,Recap" {
		t.Errorf("steps = %v, want the applicable step kept", got)
	}
	// Resolved up front, it must not also auto-skip itself as step 0.
	if kept.steps[0].AutoSkip != nil {
		t.Error("a step resolved up front should carry no auto-skip")
	}
}

// TestBuildKeepsALaterConditionalStepConditional: with something before it, the
// step decides on entry — so it re-evaluates against what the user just answered.
func TestBuildKeepsALaterConditionalStepConditional(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{
		textStep("a"),
		{
			Kind: flow.StepSelect, Key: "c", Label: "Conditional",
			Skip:  func(flow.Answers) (bool, string) { return true, "later" },
			Build: func(flow.Answers) (flow.StepContent, error) { return flow.StepContent{}, nil },
		},
		recapStep("r"),
	}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(plan.steps); strings.Join(got, ",") != "Text a,Conditional,Recap" {
		t.Errorf("steps = %v, want the conditional step kept for entry-time decision", got)
	}
	if plan.steps[1].AutoSkip == nil {
		t.Error("a conditional step that is not first must decide on entry")
	}
}

// TestBuildSurfacesAStepThatCannotBeBuilt: a picker with nothing to offer must
// refuse before anything is displayed, not show an empty list.
func TestBuildSurfacesAStepThatCannotBeBuilt(t *testing.T) {
	refusal := errors.New("nothing to pick")
	_, err := build(flow.Session{Steps: []flow.Step{{
		Kind: flow.StepSelect, Key: "a", Label: "Picker",
		Build: func(flow.Answers) (flow.StepContent, error) { return flow.StepContent{}, refusal },
	}}})
	if !errors.Is(err, refusal) {
		t.Fatalf("err = %v, want the step's own refusal", err)
	}
}

func TestBuildRejectsAKindItCannotRender(t *testing.T) {
	_, err := build(flow.Session{Steps: []flow.Step{{Kind: flow.StepKind(99), Key: "a", Label: "Odd"}}})
	if err == nil {
		t.Fatal("expected an unrenderable kind to be refused rather than guessed")
	}
	if !strings.Contains(err.Error(), "a") {
		t.Errorf("error %q should name the step", err)
	}
}

// TestRecapAppendsTheCancelRow: every recap carries the one explicit cancellation
// point, and choosing it aborts the run.
func TestRecapAppendsTheCancelRow(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	view := plan.steps[0].Model.(components.SelectListModel).View()
	if !strings.Contains(view, domain.WizardCancelLabel) {
		t.Errorf("recap should offer %q:\n%s", domain.WizardCancelLabel, view)
	}
	if !plan.steps[0].Recap {
		t.Error("a recap step should read as the review point")
	}
}

// TestReadAnswersTheWholeSession drives the wizard headlessly and checks every
// answer comes back: the typed text, the chosen option, and the recap action.
func TestReadAnswersTheWholeSession(t *testing.T) {
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"preset": "kept"}),
		Steps: []flow.Step{
			textStep("name"),
			selectStep("choice", "first", "second"),
			{Kind: flow.StepText, Key: "preset", Label: "Preset"},
			recapStep("r"),
		},
	}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	for _, r := range "feat/x" {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // name → choice
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyDown})  // second
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // choice → recap
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := answers.Value("name"); got != "feat/x" {
		t.Errorf("name = %q, want the typed value", got)
	}
	if got := answers.Value("choice"); got != "second" {
		t.Errorf("choice = %q, want the chosen option", got)
	}
	if got := answers.Value("preset"); got != "kept" {
		t.Errorf("preset = %q, want it preserved", got)
	}
	if got := answers.Value("r"); got != "go" {
		t.Errorf("recap = %q, want the confirmation", got)
	}
	if !answers.Answered("choice") {
		t.Error("a chosen option was answered by a human")
	}
	if answers.Answered("preset") {
		t.Error("a preset is not an answer a human gave")
	}
}

// TestReadTurnsTheCancelRowIntoAnAbort: choosing "No, cancel" cancels the whole
// run, wherever it was chosen.
func TestReadTurnsTheCancelRowIntoAnAbort(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyDown}) // past the separator
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := plan.read(wizard); !errors.Is(err, domain.ErrUserAborted) {
		t.Fatalf("err = %v, want ErrUserAborted", err)
	}
}

// TestAnswersFromFeedsLaterSteps: a step's Build sees what the earlier steps hold,
// which is how create's source step knows the branch already exists.
func TestAnswersFromFeedsLaterSteps(t *testing.T) {
	var seen string
	session := flow.Session{Steps: []flow.Step{
		textStep("name"),
		{
			Kind: flow.StepSelect, Key: "derived", Label: "Derived",
			Build: func(answers flow.Answers) (flow.StepContent, error) {
				seen = answers.Value("name")
				return flow.StepContent{Options: []flow.Option{{Label: "ok", Value: "ok"}}}, nil
			},
		},
	}}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	for _, r := range "feat/y" {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // advance into the derived step

	if seen != "feat/y" {
		t.Errorf("the later step saw %q, want the earlier answer", seen)
	}
}

// TestSummarizeUsesTheFlowWording: a flow can label an answer in its own words
// rather than echoing the raw value.
func TestSummarizeUsesTheFlowWording(t *testing.T) {
	step := selectStep("a", "ff")
	step.Summarize = func(answer flow.Answer) string {
		if answer.Value == "ff" {
			return "fast-forward to origin"
		}
		return "keep"
	}
	plan, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := plan.steps[0].Summary(plan.steps[0].Model); got != "fast-forward to origin" {
		t.Errorf("summary = %q, want the flow's own wording", got)
	}
}

// TestInteractivePrompterReportsItself keeps the capability honest: this prompter
// can ask, so post-execution recoveries may be offered.
func TestInteractivePrompterReportsItself(t *testing.T) {
	if !New(Params{}).Interactive() {
		t.Error("the wizard prompter must report that it can ask")
	}
}

func update(m components.WizardModel, msg tea.Msg) components.WizardModel {
	model, _ := m.Update(msg)
	updated, ok := model.(components.WizardModel)
	if !ok {
		panic("wizard Update returned a foreign model")
	}
	return updated
}
