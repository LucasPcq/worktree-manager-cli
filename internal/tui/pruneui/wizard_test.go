package pruneui

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// prevWorktrees builds the prior steps a reparent step sees: just the worktrees
// multi-select (reparent now precedes the confirm recap).
func prevWorktrees() []components.Step {
	return []components.Step{
		{Name: stepWorktrees, Model: components.NewMultiSelect(components.NewMultiSelectParams{})},
	}
}

func TestReparentStepAppliesWhenChildrenExist(t *testing.T) {
	preview := func(chosen []string, force bool) []domain.ReparentResult {
		return []domain.ReparentResult{{Branch: "child", OldParent: "p", NewParent: "gp"}}
	}
	step := reparentStep(preview)
	step.Build(prevWorktrees())
	if step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should apply when children exist")
	}
}

func TestReparentStepSkipsWithoutChildren(t *testing.T) {
	preview := func(chosen []string, force bool) []domain.ReparentResult { return nil }
	step := reparentStep(preview)
	step.Build(prevWorktrees())
	if !step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should be skipped when there are no children to reparent")
	}
	if step.SkipReason() == "" {
		t.Error("expected a skip reason when there are no children")
	}
}

func TestReparentProposalTextListsMoves(t *testing.T) {
	text := reparentProposalText([]domain.ReparentResult{
		{Branch: "child", OldParent: "old", NewParent: "new"},
	})
	if !strings.Contains(text, "child") || !strings.Contains(text, "new") || !strings.Contains(text, "old") {
		t.Errorf("proposal text missing move details: %q", text)
	}
}
