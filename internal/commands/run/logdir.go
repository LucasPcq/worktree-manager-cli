package run

import (
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type jobLogDirParams struct {
	StateDir string
	Dir      string
}

// jobLogDir resolves where the daemon persists this worktree's job logs. The
// branch is looked up here rather than passed along by the daemon, which must
// never run git; a worktree with no branch, or one git cannot name, persists
// nothing rather than sharing another's directory.
func jobLogDir(params jobLogDirParams) string {
	branch, err := worktree.CurrentBranch(worktree.CurrentBranchParams{Dir: params.Dir})
	if err != nil {
		return ""
	}
	return rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: params.StateDir, Branch: branch})
}
