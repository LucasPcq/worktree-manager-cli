package worktree

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// branchExists reports whether a local branch is still present in the repo.
func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list %s: %s: %v", branch, out, err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func TestApplyReparentsMovesEachToItsParent(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "dev/b", domain.WorktreeMetadata{SourceBranch: "dev/a", CreatedAt: "x"})
	seedMeta(t, stateDir, "dev/c", domain.WorktreeMetadata{SourceBranch: "dev/x", CreatedAt: "x"})

	applied, err := ApplyReparents(ApplyReparentsParams{
		StateDir: stateDir,
		Reparents: []domain.ReparentResult{
			{Branch: "dev/b", OldParent: "dev/a", NewParent: "main"},
			{Branch: "dev/c", OldParent: "dev/x", NewParent: "feat"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied, got %d", len(applied))
	}

	b, _ := loadMetadata(stateDir, "dev/b")
	if b.SourceBranch != "main" {
		t.Errorf("dev/b parent = %q, want main", b.SourceBranch)
	}
	c, _ := loadMetadata(stateDir, "dev/c")
	if c.SourceBranch != "feat" {
		t.Errorf("dev/c parent = %q, want feat", c.SourceBranch)
	}
}

func TestPruneRemovesWorktreesReparentsAndIsIdempotent(t *testing.T) {
	// Exercise the real (non-dry-run) destructive path: Clean removes each selected
	// worktree + its branch, surviving children reparent onto the grandparent, and
	// a second run over the same plan is a no-op success (absent worktrees tolerated).
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	featPath := filepath.Join(t.TempDir(), "feat")
	childPath := filepath.Join(t.TempDir(), "child")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", featPath, "HEAD")
	gitRun(t, source, "worktree", "add", "-q", "-b", "child", childPath, "HEAD")
	seedMeta(t, stateDir, "child", domain.WorktreeMetadata{SourceBranch: "feat", CreatedAt: "x"})

	params := domain.PruneParams{ProjectDir: source, StateDir: stateDir, BaseBranch: "main"}
	plan := domain.PrunePlan{
		Selected:  []domain.PruneCandidate{{Branch: "feat", Path: featPath, Reason: domain.PruneReasonPRMerged}},
		Reparents: []domain.ReparentResult{{Branch: "child", OldParent: "feat", NewParent: "main"}},
	}

	result, err := Prune(params, plan)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0].Branch != "feat" {
		t.Errorf("expected feat pruned, got %+v", result.Pruned)
	}
	if len(result.Reparented) != 1 || result.Reparented[0].NewParent != "main" {
		t.Errorf("expected child reparented onto main, got %+v", result.Reparented)
	}

	if fileExists(featPath, "") {
		t.Errorf("feat worktree directory should be removed")
	}
	if branchExists(t, source, "feat") {
		t.Errorf("feat branch should be deleted")
	}
	if branchExists(t, source, "child") == false {
		t.Errorf("child branch should survive")
	}
	if meta, _ := loadMetadata(stateDir, "child"); meta.SourceBranch != "main" {
		t.Errorf("child parent = %q, want main", meta.SourceBranch)
	}

	// Idempotent: the worktree is already gone, so a re-run tolerates the absence
	// and succeeds without error.
	if _, err := Prune(params, plan); err != nil {
		t.Fatalf("second Prune should be idempotent, got %v", err)
	}
}

func TestPruneDryRunReturnsPlanWithoutTouchingMetadata(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "child", domain.WorktreeMetadata{SourceBranch: "feat", CreatedAt: "x"})

	plan := domain.PrunePlan{
		Selected:  []domain.PruneCandidate{{Branch: "feat", Reason: domain.PruneReasonPRMerged}},
		Reparents: []domain.ReparentResult{{Branch: "child", OldParent: "feat", NewParent: "main"}},
		Skipped:   []domain.PruneSkip{{Branch: "dirty", Reason: domain.PruneSkipDirty}},
	}

	result, err := Prune(domain.PruneParams{StateDir: stateDir, DryRun: true}, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DryRun {
		t.Errorf("expected DryRun result")
	}
	if len(result.Pruned) != 1 || result.Pruned[0].Branch != "feat" {
		t.Errorf("expected feat in pruned, got %+v", result.Pruned)
	}
	if len(result.Reparented) != 1 || result.Reparented[0].Branch != "child" {
		t.Errorf("expected child in reparented, got %+v", result.Reparented)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %+v", result.Skipped)
	}

	// Dry run must not rewrite metadata.
	meta, _ := loadMetadata(stateDir, "child")
	if meta.SourceBranch != "feat" {
		t.Errorf("dry run mutated metadata: child parent = %q, want feat", meta.SourceBranch)
	}
}

// TestPlanPruneGHBased drives the full classification path (List → ClassifyPrune)
// with injected PR state: a worktree whose PR is merged is flagged under --merged,
// while a freshly-created worktree with no PR is never flagged — the LUC-111 fix
// (detection is GitHub-truth, not local commit topology).
func TestPlanPruneGHBased(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	mergedPath := filepath.Join(t.TempDir(), "feat-merged")
	freshPath := filepath.Join(t.TempDir(), "feat-fresh")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat-merged", mergedPath, "HEAD")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat-fresh", freshPath, "HEAD")

	params := domain.PruneParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Config:     domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BaseBranch: "main"}}},
		BaseBranch: "main",
		Merged:     true,
	}
	prs := []domain.PRInfo{{Branch: "feat-merged", State: domain.PRStateMerged}}

	plan, err := PlanPrune(params, prs)
	if err != nil {
		t.Fatalf("PlanPrune: %v", err)
	}

	sel := map[string]string{}
	for _, c := range plan.Selected {
		sel[c.Branch] = c.Reason
	}
	if sel["feat-merged"] != domain.PruneReasonPRMerged {
		t.Errorf("feat-merged should be flagged PR merged, selected=%v", sel)
	}
	if _, ok := sel["feat-fresh"]; ok {
		t.Errorf("feat-fresh (no PR) must never be flagged merged")
	}
}
