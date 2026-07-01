package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

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

func TestPruneDryRunReturnsPlanWithoutTouchingMetadata(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "child", domain.WorktreeMetadata{SourceBranch: "feat", CreatedAt: "x"})

	plan := domain.PrunePlan{
		Selected:  []domain.PruneCandidate{{Branch: "feat", Reason: domain.PruneReasonMerged}},
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
