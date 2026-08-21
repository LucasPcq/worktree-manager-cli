package rules

import (
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeJobEnvParams struct {
	Branch  string
	Ordinal int
	// PortOffsetBlock spaces two worktrees' ports apart. Zero falls back to the
	// default block rather than collapsing every worktree onto offset 0.
	PortOffsetBlock int
	// ComposeProject is the COMPOSE_PROJECT_NAME the caller's own environment
	// already defines. Non-empty wins: a project name set on purpose is an
	// answer, not a value to overwrite.
	ComposeProject string
}

// WorktreeJobEnv is what every job and lifecycle hook learns about the worktree
// it runs in. It is resolved client-side, by the only process that can ask git
// which worktree this is.
func WorktreeJobEnv(params WorktreeJobEnvParams) map[string]string {
	block := params.PortOffsetBlock
	if block <= 0 {
		block = domain.PortOffsetBlock
	}

	slug := WorktreeSlug(params.Branch)
	composeProject := params.ComposeProject
	if composeProject == "" {
		composeProject = slug
	}

	return map[string]string{
		domain.EnvWorktree:           slug,
		domain.EnvBranch:             params.Branch,
		domain.EnvOrdinal:            strconv.Itoa(params.Ordinal),
		domain.EnvPortOffset:         strconv.Itoa(params.Ordinal * block),
		domain.EnvComposeProjectName: composeProject,
	}
}
