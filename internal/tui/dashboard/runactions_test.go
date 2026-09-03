package dashboard

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
)

// A row is a worktree. Asking again which one, right after it was picked, is
// the question the positional-subject rule exists to not ask.
func TestARunStartedFromARowNamesItsWorktree(t *testing.T) {
	selected := domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}

	named, err := target.Named(target.ResolveParams{
		ProjectDir: t.TempDir(),
		Query:      runWorktree(selected),
	})

	// The repository is empty, so the name resolves to nothing — but it must
	// have been looked up as a name. A path is refused before that, which is the
	// regression this guards.
	if err == nil {
		t.Fatalf("named = %+v, want the empty repository to answer nothing", named)
	}
	if !strings.Contains(err.Error(), selected.Branch) {
		t.Fatalf("error = %v, want the branch quoted: the positional is resolved by name", err)
	}
}
