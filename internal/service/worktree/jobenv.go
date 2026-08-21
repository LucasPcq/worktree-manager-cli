package worktree

import (
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type JobEnvParams struct {
	ProjectDir string
	StateDir   string
	// Dir is the directory the command was launched from, which may be any
	// subdirectory of the worktree.
	Dir string
}

// JobEnv resolves what a worktree's jobs and hooks learn about it. Only a
// client can build this: it takes git to name the branch, and the daemon must
// never run git.
//
// A user-defined COMPOSE_PROJECT_NAME is read from this process's environment
// and passed through untouched — a project name set on purpose is an answer.
func JobEnv(params JobEnvParams) (map[string]string, error) {
	branch, err := CurrentBranch(CurrentBranchParams{Dir: params.Dir})
	if err != nil {
		return nil, err
	}
	return BranchEnv(BranchEnvParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     branch,
	})
}

type BranchEnvParams struct {
	ProjectDir string
	StateDir   string
	Branch     string
}

// BranchEnv is JobEnv for a caller that already knows the branch — the
// lifecycle hooks, which are handed one rather than a directory to ask git
// about.
func BranchEnv(params BranchEnvParams) (map[string]string, error) {
	ordinal, err := EnsureOrdinal(EnsureOrdinalParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return nil, err
	}

	return rules.WorktreeJobEnv(rules.WorktreeJobEnvParams{
		Branch:          params.Branch,
		Ordinal:         ordinal,
		PortOffsetBlock: domain.PortOffsetBlock,
		ComposeProject:  os.Getenv(domain.EnvComposeProjectName),
	}), nil
}

// hookEnv resolves the worktree variables a lifecycle hook runs with, degrading
// to nothing rather than to another worktree's values.
func hookEnv(params BranchEnvParams) map[string]string {
	env, err := BranchEnv(params)
	if err != nil {
		return nil
	}
	return env
}
