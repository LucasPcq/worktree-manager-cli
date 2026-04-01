package worktree

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSortStatusesParentFirst(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "feature", IsParent: false, CreatedAt: time.Now()},
		{Branch: "main", IsParent: true, CreatedAt: time.Now()},
	}

	sortStatuses(statuses)

	if statuses[0].Branch != "main" {
		t.Errorf("expected parent (main) first, got %s", statuses[0].Branch)
	}
}

func TestSortStatusesByCreationDate(t *testing.T) {
	now := time.Now()

	statuses := []domain.WorktreeStatus{
		{Branch: "newer", IsParent: false, CreatedAt: now.Add(2 * time.Hour)},
		{Branch: "main", IsParent: true, CreatedAt: now},
		{Branch: "older", IsParent: false, CreatedAt: now.Add(1 * time.Hour)},
	}

	sortStatuses(statuses)

	if statuses[0].Branch != "main" {
		t.Errorf("expected main first, got %s", statuses[0].Branch)
	}
	if statuses[1].Branch != "older" {
		t.Errorf("expected older second, got %s", statuses[1].Branch)
	}
	if statuses[2].Branch != "newer" {
		t.Errorf("expected newer third, got %s", statuses[2].Branch)
	}
}

func TestWorktreeCreatedAtFallbackMtime(t *testing.T) {
	dir := t.TempDir()

	created := worktreeCreatedAt(dir)
	if created.IsZero() {
		t.Error("expected non-zero time from mtime fallback")
	}
}

func TestWorktreeCreatedAtFromMeta(t *testing.T) {
	dir := t.TempDir()
	metadata := domain.WorktreeMetadata{
		SourceBranch: "main",
		CreatedAt:    "2026-04-01T12:00:00Z",
		EnvStrategy:  domain.EnvStrategyExample,
	}
	_ = writeMetadata(dir, metadata)

	created := worktreeCreatedAt(dir)
	expected, _ := time.Parse(time.RFC3339, "2026-04-01T12:00:00Z")

	if !created.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, created)
	}
}
