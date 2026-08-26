package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

// everyStepModel is one instance of each model a Step may carry. The wizard
// dispatches on the concrete type in half a dozen switches; a model missing from
// one of them renders blank and swallows every key, so this table is what keeps
// a new model from being wired into some of them only.
func everyStepModel() []Step {
	const desc = "the description"
	return []Step{
		{Name: "select", Model: NewSelectList(NewSelectListParams{Title: "t", Description: desc, Items: []SelectItem{{Label: "a"}}})},
		{Name: "text", Model: NewTextInput(NewTextInputParams{Title: "t", Description: desc})},
		{Name: "confirm", Model: NewConfirm(NewConfirmParams{Title: "t", Description: desc})},
		{Name: "multiselect", Model: NewMultiSelect(NewMultiSelectParams{Title: "t", Description: desc, Items: []MultiSelectItem{{Label: "a"}}})},
		{Name: "reorder", Model: NewReorderList(NewReorderListParams{Title: "t", Description: desc, Items: []ReorderItem{{Label: "a"}}})},
		{Name: "hooks", Model: NewHookList(NewHookListParams{Title: "t", Description: desc, Hooks: []domain.HookCommand{{Cmd: "echo"}}})},
		{Name: "env", Model: NewEnvResolve(NewEnvResolveParams{Title: "t", Description: desc, Files: []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{{Key: "PORT", Status: domain.EnvKeyMissing}}}}}})},
		{Name: "ports", Model: NewPortList(NewPortListParams{Title: "t", Description: desc, Entries: []domain.PortEntry{{Job: "web", Name: "PORT", Base: 3000}}})},
		{Name: "kinds", Model: NewKindList(NewKindListParams{Title: "t", Description: desc, Entries: []domain.JobKindChoice{{Label: "root / build", Cmd: "turbo run build", Name: "build", Kind: domain.JobKindTask}}})},
		{Name: "profiles", Model: NewProfileList(NewProfileListParams{Title: "t", Description: desc, Profiles: []domain.ProfileConfig{{Name: "all"}}})},
	}
}

func TestWizardRendersEveryStepModel(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			if view := m.viewStep(0); view == "" {
				t.Error("the step rendered nothing: its type is missing from viewStep")
			}
		})
	}
}

func TestWizardRoutesEscapeToEveryStepModel(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			s := step
			_, back, _ := (WizardModel{}).updateStep(&s, key(tea.KeyEsc))
			if !back {
				t.Error("escape did nothing: the type is missing from updateStep, so no key reaches the step")
			}
		})
	}
}

func TestWizardReadsEveryStepDescription(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			if got := m.stepDescription(step); got != "the description" {
				t.Errorf("description = %q, want the one it was built with", got)
			}
		})
	}
}
