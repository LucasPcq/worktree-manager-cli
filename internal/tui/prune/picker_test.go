package prune

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// prevWithConfirm builds the prior steps a reparent step sees: a worktrees
// multi-select and the confirm step (which defaults to "Yes, prune").
func prevWithConfirm() []components.Step {
	return []components.Step{
		{Name: stepWorktrees, Model: components.NewMultiSelect(components.NewMultiSelectParams{})},
		{Name: stepConfirm, Model: confirmStep(nil, domain.PrunePlan{})},
	}
}

func TestReparentStepAppliesWhenChildrenExist(t *testing.T) {
	preview := func(chosen []string, force bool) []domain.ReparentResult {
		return []domain.ReparentResult{{Branch: "child", OldParent: "p", NewParent: "gp"}}
	}
	step := reparentStep(preview)
	step.Build(prevWithConfirm()) // confirm defaults to "yes" → not cancelled
	if step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should apply when children exist and prune is confirmed")
	}
}

func TestReparentStepSkipsWithoutChildren(t *testing.T) {
	preview := func(chosen []string, force bool) []domain.ReparentResult { return nil }
	step := reparentStep(preview)
	step.Build(prevWithConfirm())
	if !step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should be skipped when there are no children to reparent")
	}
}

func TestReparentProposalTextListsMoves(t *testing.T) {
	text := reparentProposalText([]domain.ReparentResult{
		{Branch: "child", OldParent: "old", NewParent: "new"},
	})
	if !strings.Contains(text, "child") || !strings.Contains(text, "old → new") {
		t.Errorf("proposal text missing move details: %q", text)
	}
}
