package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func indexOf(steps []domain.SyncStep, branch string) int {
	for i, s := range steps {
		if s.Branch == branch {
			return i
		}
	}
	return -1
}

func TestBuildSyncPlanLinearChain(t *testing.T) {
	plan, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "dev1", SourceBranch: "feat"},
			{Branch: "feat", SourceBranch: "main"},
			{Branch: "main", IsMain: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
	if indexOf(plan.Steps, "feat") > indexOf(plan.Steps, "dev1") {
		t.Fatalf("feat must come before dev1: %+v", plan.Steps)
	}
	if indexOf(plan.Steps, "main") != -1 {
		t.Fatalf("base branch must not be a step")
	}
}

func TestBuildSyncPlanFanOut(t *testing.T) {
	plan, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "dev1", SourceBranch: "feat"},
			{Branch: "dev2", SourceBranch: "feat"},
			{Branch: "feat", SourceBranch: "main"},
			{Branch: "main", IsMain: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	feat := indexOf(plan.Steps, "feat")
	if feat > indexOf(plan.Steps, "dev1") || feat > indexOf(plan.Steps, "dev2") {
		t.Fatalf("feat must precede both children: %+v", plan.Steps)
	}
}

func TestBuildSyncPlanMultipleRoots(t *testing.T) {
	plan, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "featA", SourceBranch: "main"},
			{Branch: "featB", SourceBranch: "main"},
			{Branch: "main", IsMain: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 root steps, got %d", len(plan.Steps))
	}
}

func TestBuildSyncPlanCycle(t *testing.T) {
	_, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "a", SourceBranch: "b"},
			{Branch: "b", SourceBranch: "a"},
		},
	})
	if err == nil {
		t.Fatal("expected a cycle error")
	}
}

func TestBuildSyncPlanUnknownParent(t *testing.T) {
	plan, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "orphan", SourceBranch: ""},
			{Branch: "main", IsMain: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Branch != "orphan" {
		t.Fatalf("expected orphan step, got %+v", plan.Steps)
	}
	if plan.Steps[0].SourceBranch != "" {
		t.Fatalf("expected empty source branch for orphan")
	}
}

func TestFilterSyncStepsPreservesOrder(t *testing.T) {
	steps := []domain.SyncStep{
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "mid", SourceBranch: "feat"},
		{Branch: "deep", SourceBranch: "mid"},
	}

	filtered := FilterSyncSteps(steps, []string{"deep", "feat"})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 steps, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Branch != "feat" || filtered[1].Branch != "deep" {
		t.Fatalf("expected topological order feat then deep, got %+v", filtered)
	}
}

func TestFilterSyncStepsEmptySelectionReturnsAll(t *testing.T) {
	steps := []domain.SyncStep{{Branch: "feat"}, {Branch: "mid"}}
	if got := FilterSyncSteps(steps, nil); len(got) != 2 {
		t.Fatalf("nil selection must return all steps, got %+v", got)
	}
	if got := FilterSyncSteps(steps, []string{}); len(got) != 2 {
		t.Fatalf("empty selection must return all steps, got %+v", got)
	}
}

func TestFilterSyncStepsIgnoresUnknownBranches(t *testing.T) {
	steps := []domain.SyncStep{{Branch: "feat"}}
	filtered := FilterSyncSteps(steps, []string{"main", "nope"})
	if len(filtered) != 0 {
		t.Fatalf("expected no steps for branches without a matching step, got %+v", filtered)
	}
}

func TestBuildSyncPlanDeepBeforeShallow(t *testing.T) {
	plan, err := BuildSyncPlan(BuildSyncPlanParams{
		BaseBranch: "main",
		Nodes: []domain.WorktreeNode{
			{Branch: "deep", SourceBranch: "mid"},
			{Branch: "mid", SourceBranch: "feat"},
			{Branch: "feat", SourceBranch: "main"},
			{Branch: "main", IsMain: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexOf(plan.Steps, "feat") > indexOf(plan.Steps, "mid") ||
		indexOf(plan.Steps, "mid") > indexOf(plan.Steps, "deep") {
		t.Fatalf("expected feat < mid < deep ordering: %+v", plan.Steps)
	}
}

func TestParentsOutsideCascade(t *testing.T) {
	// feat is a step, so it refreshes itself; main is the base, refreshed by
	// updateBase. Only feature — a parent no step covers — is left, carrying the
	// two steps that depend on it.
	steps := []domain.SyncStep{
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "dev", SourceBranch: "feature"},
		{Branch: "other", SourceBranch: "feature"},
		{Branch: "child", SourceBranch: "feat"},
		{Branch: "orphan", SourceBranch: ""},
	}

	parents := ParentsOutsideCascade(ParentsOutsideCascadeParams{Steps: steps, BaseBranch: "main"})

	if len(parents) != 1 {
		t.Fatalf("parents = %+v, want only feature", parents)
	}
	if parents[0].Branch != "feature" {
		t.Fatalf("parent = %q, want feature", parents[0].Branch)
	}
	if got := parents[0].Children; len(got) != 2 || got[0] != "dev" || got[1] != "other" {
		t.Fatalf("children = %v, want [dev other]", got)
	}
}

func TestParentsOutsideCascadeEmptyWhenAllCovered(t *testing.T) {
	steps := []domain.SyncStep{
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "dev", SourceBranch: "feat"},
	}
	if parents := ParentsOutsideCascade(ParentsOutsideCascadeParams{Steps: steps, BaseBranch: "main"}); len(parents) != 0 {
		t.Fatalf("parents = %+v, want none", parents)
	}
}

func TestParentBranches(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat", SourceBranch: "main"}, // base parent: excluded
		{Branch: "dev-1", SourceBranch: "feature"},
		{Branch: "dev-2", SourceBranch: "feature"}, // same parent: deduped
		{Branch: "orphan", SourceBranch: ""},       // unset parent: excluded
		{Branch: "deep", SourceBranch: "feat"},
	}

	got := ParentBranches(ParentBranchesParams{Nodes: nodes, BaseBranch: "main"})

	if len(got) != 2 || got[0] != "feature" || got[1] != "feat" {
		t.Fatalf("ParentBranches = %v, want [feature feat]", got)
	}
}

func TestSprintSyncPlan_EmptyPlan(t *testing.T) {
	if got := SprintSyncPlan(domain.SyncPlan{BaseBranch: "main"}); got != "" {
		t.Fatalf("empty plan should render empty, got %q", got)
	}
}

func TestSprintSyncPlan_ListsStepsPlain(t *testing.T) {
	plan := domain.SyncPlan{
		BaseBranch:   "main",
		BaseTargeted: true,
		Steps: []domain.SyncStep{
			{Branch: "feat/a", SourceBranch: "main"},
			{Branch: "feat/b", SourceBranch: "feat/a"},
		},
	}

	got := SprintSyncPlan(plan)

	want := "Sync plan\n1. feat/a ← main\n2. feat/b ← feat/a"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	// Plain output: no ANSI escapes, so it can be re-styled inside a wizard desc.
	if strings.Contains(got, "\x1b[") {
		t.Errorf("expected plain (unstyled) output, got ANSI escapes: %q", got)
	}
}

func TestSprintSyncPlan_UnknownParent(t *testing.T) {
	plan := domain.SyncPlan{
		BaseBranch: "main",
		Steps:      []domain.SyncStep{{Branch: "feat/a"}},
	}
	if got := SprintSyncPlan(plan); !strings.Contains(got, "unknown parent") {
		t.Fatalf("expected 'unknown parent' fallback, got %q", got)
	}
}

// A cascade where every step rebases onto some other parent never touches the
// base, so the header may not name it.
func TestSprintSyncPlanOmitsUntargetedBase(t *testing.T) {
	plan := domain.SyncPlan{
		BaseBranch: "main",
		Steps: []domain.SyncStep{
			{Branch: "dev-1", SourceBranch: "feature"},
			{Branch: "dev-2", SourceBranch: "feature"},
		},
	}
	got := SprintSyncPlan(plan)
	if strings.Contains(got, "main") {
		t.Errorf("plan header must not name an untargeted base, got:\n%s", got)
	}
	if !strings.HasPrefix(got, "Sync plan\n") {
		t.Errorf("plan should still be titled, got:\n%s", got)
	}
}

func TestStaleParentsFor(t *testing.T) {
	classified := []domain.ParentUpdate{
		{Branch: "feature", Status: domain.ParentBehind, OldTip: "aaa"},
		{Branch: "split", Status: domain.ParentDiverged},
	}
	uncovered := []domain.ParentUpdate{
		{Branch: "feature", Children: []string{"dev-1", "dev-2"}},
		{Branch: "split", Children: []string{"dev-3"}},
		{Branch: "unknown", Children: []string{"dev-4"}},
	}

	stale := StaleParentsFor(StaleParentsForParams{Uncovered: uncovered, Classified: classified})

	// Only a parent a fast-forward would actually advance: diverged and unseen are out.
	if len(stale) != 1 || stale[0].Branch != "feature" {
		t.Fatalf("StaleParentsFor = %+v, want only feature", stale)
	}
	if stale[0].OldTip != "aaa" {
		t.Errorf("tips should come from the inspection, got %q", stale[0].OldTip)
	}
	if got := stale[0].Children; len(got) != 2 || got[0] != "dev-1" {
		t.Errorf("children should come from the selection, got %v", got)
	}
}
