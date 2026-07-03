package wt

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestResolveSyncSelectionRequiresTargetWhenNoPrompt covers the hardened bypass
// path --yes routes to: with no picker available (CanPrompt false, as under --yes /
// no TTY / JSON) and no worktree fixed by args/--all, a missing selection errors
// naming --all instead of falling back to a picker.
func TestResolveSyncSelectionRequiresTargetWhenNoPrompt(t *testing.T) {
	_, err := resolveSyncSelection(resolveSyncSelectionParams{CanPrompt: false})
	if err == nil {
		t.Fatal("expected an error when no worktree is selected and prompting is disabled")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("expected the error to mention --all, got: %v", err)
	}

	// With worktrees fixed by --all, the same no-prompt path resolves without error.
	sel, err := resolveSyncSelection(resolveSyncSelectionParams{CanPrompt: false, All: true})
	if err != nil {
		t.Fatalf("resolveSyncSelection with --all: %v", err)
	}
	if sel.Branches != nil {
		t.Errorf("--all should sync every worktree (nil branches), got %v", sel.Branches)
	}
}

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
