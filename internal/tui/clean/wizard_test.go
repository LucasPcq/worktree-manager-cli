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
	desc := deleteDescription(check, "")
	for _, want := range []string{"uncommitted changes", "2 commit(s)", "http://pr", "Will delete:", "/w/feat", "feat"} {
		if !strings.Contains(desc, want) {
			t.Errorf("description missing %q:\n%s", want, desc)
		}
	}
}

func TestDeleteStepOffersForceOnlyWhenUnsafe(t *testing.T) {
	safe := deleteStep(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b"}, "")
	unsafe := deleteStep(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b", IsDirty: true}, "")
	// The force option only appears for an unsafe worktree, adding rows.
	if len(unsafe.View()) <= len(safe.View()) {
		t.Error("expected the force option to appear only when the worktree is unsafe")
	}
}

func TestReparentStepAppliesWithChildren(t *testing.T) {
	plan := domain.CleanReparentPlan{
		Grandparent: "gp",
		Children:    []domain.ReparentResult{{Branch: "c", OldParent: "o", NewParent: "gp"}},
	}
	step := reparentStep(func(string) domain.CleanReparentPlan { return plan })
	step.Build(nil)
	if step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should apply when the branch has orphaned children")
	}
	if step.SkipReason() != "" {
		t.Errorf("no skip reason expected when applying, got %q", step.SkipReason())
	}
}

func TestReparentStepSkipsWithoutChildren(t *testing.T) {
	step := reparentStep(func(string) domain.CleanReparentPlan { return domain.CleanReparentPlan{} })
	step.Build(nil)
	if !step.AutoSkip(components.WizardModel{}) {
		t.Error("reparent step should be skipped when there are no children")
	}
	if step.SkipReason() == "" {
		t.Error("expected a skip reason when there are no children")
	}
}

func TestReparentProposalTextListsMoves(t *testing.T) {
	text := reparentProposalText(domain.CleanReparentPlan{
		Grandparent: "gp",
		Children:    []domain.ReparentResult{{Branch: "child", OldParent: "old", NewParent: "gp"}},
	})
	if !strings.Contains(text, "child") || !strings.Contains(text, "gp") || !strings.Contains(text, "old") {
		t.Errorf("proposal text missing move details: %q", text)
	}
}
