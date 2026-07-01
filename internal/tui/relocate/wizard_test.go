package relocate

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func localCandidates(names ...string) *[]domain.BranchCandidate {
	out := make([]domain.BranchCandidate, 0, len(names))
	for _, n := range names {
		out = append(out, domain.BranchCandidate{Name: n})
	}
	return &out
}

func TestBuildRecapGroupsAndResolvesParents(t *testing.T) {
	plan := domain.RelocatePlan{
		BasePath: "../.trees",
		Steps: []domain.RelocateStep{
			{Branch: "hotfix", ToPath: "/repo/../.trees/hotfix", Status: domain.RelocateStatusMove, Adopt: true, Parent: "main"},
			{Branch: "legacy", ToPath: "/repo/../.trees/legacy", Status: domain.RelocateStatusAdopt, Adopt: true, Parent: "main"},
			{Branch: "experiment", Status: domain.RelocateStatusSkippedDirty},
			{Branch: "conflicted", Status: domain.RelocateStatusBlockedDest},
		},
	}
	parents := map[string]string{"hotfix": "feature/api"} // legacy falls back to step.Parent

	recap := buildRecap(plan, parents)

	for _, want := range []string{
		"To apply:",
		"hotfix → ../.trees/hotfix (adopt, parent: feature/api)",
		"legacy  adopt in place (parent: main)",
		"Skipped:",
		"experiment — uncommitted changes",
		"Blocked:",
		"conflicted — target path occupied",
	} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q in:\n%s", want, recap)
		}
	}
}

func TestExtractParentsMapsPickersByIndex(t *testing.T) {
	adoptions := []domain.RelocateStep{
		{Branch: "hotfix"},
		{Branch: "legacy"},
	}
	steps := []components.Step{
		parentStep(parentStepParams{Branch: "hotfix", BaseBranch: "main", Holder: localCandidates("main", "feature/api")}),
		parentStep(parentStepParams{Branch: "legacy", BaseBranch: "main", Holder: localCandidates("main")}),
	}

	parents := extractParents(steps, adoptions)

	// Default selection is the first item, which is the base branch.
	if parents["hotfix"] != "main" {
		t.Errorf("hotfix parent = %q, want main", parents["hotfix"])
	}
	if parents["legacy"] != "main" {
		t.Errorf("legacy parent = %q, want main", parents["legacy"])
	}
}

func TestParentStepExcludesOwnBranch(t *testing.T) {
	step := parentStep(parentStepParams{
		Branch:     "feature/api",
		BaseBranch: "main",
		Holder:     localCandidates("main", "feature/api", "other"),
	})
	sl, ok := step.Model.(components.SelectListModel)
	if !ok {
		t.Fatalf("expected SelectListModel, got %T", step.Model)
	}
	if got := components.SelectSummary(sl); got != "main" {
		t.Errorf("default parent = %q, want main", got)
	}
}
