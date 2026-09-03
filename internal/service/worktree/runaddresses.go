package worktree

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RunAddressesForParams struct {
	ProjectDir string
	StateDir   string
	RunConfig  domain.RunConfig
	// Branches must only name worktrees that already have a job up. BranchEnv
	// goes through EnsureOrdinal, which writes: sweeping every worktree would
	// allocate an ordinal to worktrees that never ran anything.
	Branches  []string
	ProxyPort int
}

// RunAddressesFor is where every declared job answers, in each worktree asked
// for. A branch whose environment cannot be read is left out rather than given
// a guess: a wrong port reads as a truth.
func RunAddressesFor(params RunAddressesForParams) map[string]map[string]domain.JobAddress {
	if len(params.RunConfig.Jobs) == 0 {
		return nil
	}

	addresses := make(map[string]map[string]domain.JobAddress, len(params.Branches))
	for _, branch := range params.Branches {
		env, err := BranchEnv(WorktreeRef{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Branch:     branch,
		})
		if err != nil {
			continue
		}
		offset, err := strconv.Atoi(env[domain.EnvPortOffset])
		if err != nil {
			continue
		}
		addresses[branch] = rules.WorktreeJobAddresses(rules.WorktreeJobAddressesParams{
			Config:     params.RunConfig,
			PortOffset: offset,
			Worktree:   env[domain.EnvWorktree],
			Project:    filepath.Base(params.ProjectDir),
			PublicPort: params.ProxyPort,
		})
	}
	return addresses
}
