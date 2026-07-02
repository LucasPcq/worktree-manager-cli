package clean

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestDeleteDescriptionRecapsWarningsAndTarget(t *testing.T) {
	check := domain.CleanCheckResult{
		Branch:          "feat",
		WorktreePath:    "/w/feat",
		IsDirty:         true,
		UnpushedCommits: 2,
		HasOpenPR:       true,
		PRUrl:           "http://pr",
	}
	desc := deleteDescription(check)
	for _, want := range []string{"uncommitted changes", "2 commit(s)", "http://pr", "Will delete:", "/w/feat", "feat"} {
		if !strings.Contains(desc, want) {
			t.Errorf("description missing %q:\n%s", want, desc)
		}
	}
}

func TestDeleteStepOffersForceOnlyWhenUnsafe(t *testing.T) {
	safe := deleteStep(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b"})
	unsafe := deleteStep(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b", IsDirty: true})
	// The force option only appears for an unsafe worktree, adding rows.
	if len(unsafe.View()) <= len(safe.View()) {
		t.Error("expected the force option to appear only when the worktree is unsafe")
	}
}

func TestReparentStepAppliesWhenNotCancelled(t *testing.T) {
	plan := domain.CleanReparentPlan{
		Grandparent: "gp",
		Children:    []domain.ReparentResult{{Branch: "c", OldParent: "o", NewParent: "gp"}},
	}
	step := reparentStep(reparentStepParams{
		Preview:     func(string) domain.CleanReparentPlan { return plan },
		Preselected: "feat",
	})
	// Delete step defaults to "Yes, delete" → not cancelled.
	step.Build([]components.Step{{Name: stepDelete, Model: deleteStep(domain.CleanCheckResult{})}})
	if step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should apply when the delete is confirmed")
	}
}

func TestReparentStepSkipsWithoutChildren(t *testing.T) {
	step := reparentStep(reparentStepParams{
		Preview:     func(string) domain.CleanReparentPlan { return domain.CleanReparentPlan{} },
		Preselected: "feat",
	})
	step.Build([]components.Step{{Name: stepDelete, Model: deleteStep(domain.CleanCheckResult{})}})
	if !step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should be skipped when there are no children")
	}
}

func TestReparentProposalTextListsMoves(t *testing.T) {
	text := reparentProposalText(domain.CleanReparentPlan{
		Grandparent: "gp",
		Children:    []domain.ReparentResult{{Branch: "child", OldParent: "old", NewParent: "gp"}},
	})
	if !strings.Contains(text, "child") || !strings.Contains(text, "old → gp") {
		t.Errorf("proposal text missing move details: %q", text)
	}
}
