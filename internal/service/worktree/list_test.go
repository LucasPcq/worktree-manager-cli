package worktree

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestWorktreeCreatedAtFallbackMtime(t *testing.T) {
	dir := t.TempDir()

	created := worktreeCreatedAt(t.TempDir(), "feat/x", dir)
	if created.IsZero() {
		t.Error("expected non-zero time from mtime fallback")
	}
}

func TestWorktreeCreatedAtFromMeta(t *testing.T) {
	stateDir := t.TempDir()
	branch := "feat/x"
	metadata := domain.WorktreeMetadata{
		SourceBranch: "main",
		CreatedAt:    "2026-04-01T12:00:00Z",
		EnvStrategy:  domain.EnvStrategyExample,
	}
	_ = writeMetadata(rules.WorktreeMetaDir(stateDir, branch), metadata)

	created := worktreeCreatedAt(stateDir, branch, t.TempDir())
	expected, _ := time.Parse(time.RFC3339, "2026-04-01T12:00:00Z")

	if !created.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, created)
	}
}
