package rules

import (
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
