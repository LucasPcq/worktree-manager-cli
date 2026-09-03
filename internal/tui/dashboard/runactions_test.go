package dashboard

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
)

// A row is a worktree. Asking again which one, right after it was picked, is
// the question the positional-subject rule exists to not ask.
func TestARunStartedFromARowNamesItsWorktree(t *testing.T) {
	selected := domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}

	presets := target.Presets(target.PresetParams{
		Named: &target.Resolved{Dir: runWorktree(selected), Branch: selected.Branch},
	})

	if got := presets.Value(target.KeyWorktree); got != "/wt/x" {
		t.Fatalf("preset worktree = %q, want the row's own path", got)
	}
}
