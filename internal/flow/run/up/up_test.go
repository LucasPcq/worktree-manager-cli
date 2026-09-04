package up

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A flag that contradicts itself is not a decision to default: --exclusive
// stops all but one, and the run was told to bring up two.
func TestExclusiveIsRefusedOnSeveralWorktrees(t *testing.T) {
	repo := gittest.InitRepo(t)
	second := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)

	_, err := Run(Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   Request{Worktrees: []string{"main", "feature"}, Cwd: repo, Exclusive: true},
		Prompter:  flow.Unattended{},
		Presenter: presenterOnly{&flowtest.Recorder{}},
	})

	if !errors.Is(err, domain.ErrExclusiveMultiWorktree) {
		t.Fatalf("err = %v, want the contradiction refused", err)
	}
}
