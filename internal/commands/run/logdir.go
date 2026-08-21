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

type jobEnvParams struct {
	ProjectDir string
	StateDir   string
	Dir        string
}

// jobEnv resolves the worktree-scoped environment handed to every job of this
// run. Like jobLogDir it degrades to nothing rather than to another worktree's
// values: a run whose worktree cannot be named injects no isolation instead of
// the wrong one.
func jobEnv(params jobEnvParams) map[string]string {
	env, err := worktree.JobEnv(worktree.JobEnvParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Dir:        params.Dir,
	})
	if err != nil {
		return nil
	}
	return env
}
