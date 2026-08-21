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

	ordinal, err := EnsureOrdinal(EnsureOrdinalParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     branch,
	})
	if err != nil {
		return nil, err
	}

	return rules.WorktreeJobEnv(rules.WorktreeJobEnvParams{
		Branch:          branch,
		Ordinal:         ordinal,
		PortOffsetBlock: domain.PortOffsetBlock,
		ComposeProject:  os.Getenv(domain.EnvComposeProjectName),
	}), nil
}
