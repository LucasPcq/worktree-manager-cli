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
	if kept.steps[0].AutoSkip != nil {
		t.Error("a step resolved up front should carry no auto-skip")
	}
}

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

// A picker with nothing to offer must refuse before anything is displayed.
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

func multiSelectStep(key string, values ...string) flow.Step {
	options := make([]flow.Option, 0, len(values))
	for _, value := range values {
		options = append(options, flow.Option{Label: value, Value: value})
	}
	return flow.Step{Kind: flow.StepMultiSelect, Key: key, Label: "Multi " + key, Options: options}
}

func TestBuildRendersAMultiSelectStep(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{multiSelectStep("a", "one", "two"), recapStep("r")}}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	model, ok := plan.steps[0].Model.(components.MultiSelectModel)
	if !ok {
		t.Fatalf("model = %T, want a MultiSelectModel", plan.steps[0].Model)
	}
	if got := model.Values(); len(got) != 0 {
		t.Errorf("values = %v, want nothing checked before the user answers", got)
	}
}

// A set answer travels in Answer.Values; reading it as a single value would
// silently collapse the selection to nothing.
func TestAnswerOfReadsAMultiSelectAsASet(t *testing.T) {
	model := components.NewMultiSelect(components.NewMultiSelectParams{
		Items: []components.MultiSelectItem{
			{Label: "one", Value: "one", Selected: true},
			{Label: "two", Value: "two"},
			{Label: "three", Value: "three", Selected: true},
		},
	})

	answer := answerOf(flow.StepMultiSelect, model)
	if len(answer.Values) != 2 || answer.Values[0] != "one" || answer.Values[1] != "three" {
		t.Errorf("values = %v, want the checked items", answer.Values)
	}
	if !answer.Asked {
		t.Error("an answered step must read as asked")
	}
}

// The dashboard's modal refuses an unknown kind rather than guessing; the CLI
// wizard must do the same, so a new kind is heard about immediately.
func TestBuildRefusesAnUnknownKind(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{Kind: flow.StepKind(99), Key: "a", Label: "Mystery"}}}

	if _, err := build(session); err == nil {
		t.Fatal("an unsupported step kind must refuse the run")
	}
}

// The CLI wizard must carry a step's pre-checked options too: prune's picker
// opens with the safe candidates already checked, and losing that would turn a
// confirmation into a full re-selection.
func TestMultiSelectKeepsPreCheckedOptions(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
		Options: []flow.Option{
			{Label: "merged-wt", Value: "merged-wt", Selected: true},
			{Label: "dirty-wt", Value: "dirty-wt", Tag: "dirty", Tone: flow.ToneDanger},
		},
	}

	model := multiSelect(step, flow.StepContent{Options: step.Options})

	got := model.Values()
	if len(got) != 1 || got[0] != "merged-wt" {
		t.Errorf("values = %v, want only the pre-checked option", got)
	}
}
