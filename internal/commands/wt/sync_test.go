package wt

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBranchesForSync(t *testing.T) {
	// --all preserves the "sync every worktree" semantics by passing nil, even
	// though the picker previewed a resolved list.
	if got := branchesForSync(true, []string{"a", "b"}); got != nil {
		t.Errorf("branchesForSync(all=true) = %v, want nil", got)
	}
	// Otherwise the explicit list is passed through.
	got := branchesForSync(false, []string{"a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("branchesForSync(all=false) = %v, want [a b]", got)
	}
}

func TestSyncableBranchesExcludesBase(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", IsParent: true},
		{Branch: "feat-a"},
		{Branch: "feat-b"},
	}
	got := syncableBranches(statuses)
	if len(got) != 2 || got[0] != "feat-a" || got[1] != "feat-b" {
		t.Errorf("syncableBranches = %v, want [feat-a feat-b] (base excluded)", got)
	}
}

func TestPickerPreselection(t *testing.T) {
	// When the worktrees still need choosing, the picker gets nil (shows the select).
	if got := pickerPreselection(true, []string{"a"}); got != nil {
		t.Errorf("pickerPreselection(needSelect=true) = %v, want nil", got)
	}
	// Otherwise the fixed list is forwarded.
	if got := pickerPreselection(false, []string{"a"}); len(got) != 1 || got[0] != "a" {
		t.Errorf("pickerPreselection(needSelect=false) = %v, want [a]", got)
	}
}
