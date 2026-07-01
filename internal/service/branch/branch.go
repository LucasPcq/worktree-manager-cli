// Package branch orchestrates branch-candidate listing for the pickers: it lists
// local and origin branches, computes each local branch's divergence from its
// origin counterpart, and merges them into a single ordered candidate set.
package branch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ListParams holds inputs for listing branch candidates.
type ListParams struct {
	ProjectDir string
}

// Candidates returns the local and origin remote-tracking branches as a single
// ordered candidate set, with each local branch tagged with its ahead/behind
// divergence from its same-name origin counterpart. Errors are swallowed (best
// effort): a repo with no branches yet simply yields an empty list.
func Candidates(params ListParams) []domain.BranchCandidate {
	local, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil
	}
	remote, _ := infra.ListRemoteBranches(infra.ListBranchesParams{ProjectDir: params.ProjectDir})

	return rules.MergeBranchCandidates(rules.MergeBranchCandidatesParams{
		Local:      local,
		Remote:     remote,
		Divergence: divergence(divergenceParams{ProjectDir: params.ProjectDir, Local: local, Remote: remote}),
	})
}

// Refresh fetches origin then recomputes the candidate set so the divergence
// badges reflect the latest remote state. A failed fetch is ignored so the
// picker still refreshes from whatever remote-tracking refs are present.
func Refresh(params ListParams) []domain.BranchCandidate {
	_ = infra.Fetch(infra.FetchParams{ProjectDir: params.ProjectDir})
	return Candidates(params)
}

// BranchParams identifies a single local branch to inspect or update.
type BranchParams struct {
	ProjectDir string
	Branch     string
}

// Divergence classifies how a local branch relates to its origin counterpart,
// using the current remote-tracking refs without fetching. Returns
// DivergenceUnknown when the branch has no origin counterpart.
func Divergence(params BranchParams) (domain.DivergenceState, domain.AheadBehind) {
	ab, err := infra.AheadBehind(infra.AheadBehindParams{
		ProjectDir: params.ProjectDir,
		Local:      params.Branch,
		Remote:     domain.RemoteBranchPrefix + params.Branch,
	})
	if err != nil {
		return domain.DivergenceUnknown, domain.AheadBehind{}
	}
	return rules.ClassifyDivergence(ab.Ahead, ab.Behind), ab
}

// FastForwardToOrigin advances a local branch to its origin counterpart. It
// fetches first, then refuses (without modifying anything) when the branch has
// diverged, when its worktree has uncommitted changes, or when the fast-forward
// otherwise fails. A branch that is already up to date is a no-op.
func FastForwardToOrigin(params BranchParams) error {
	if err := infra.FetchBranch(infra.FetchBranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch}); err != nil {
		return err
	}

	ab, err := infra.AheadBehind(infra.AheadBehindParams{
		ProjectDir: params.ProjectDir,
		Local:      params.Branch,
		Remote:     domain.RemoteBranchPrefix + params.Branch,
	})
	if err != nil {
		return err
	}
	if ab.Ahead > 0 {
		return fmt.Errorf("%s has diverged from origin (%d ahead, %d behind)", params.Branch, ab.Ahead, ab.Behind)
	}
	if ab.Behind == 0 {
		return nil
	}

	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
	if err != nil {
		if errors.Is(err, domain.ErrWorktreeNotFound) {
			return infra.UpdateLocalBranchToRemote(infra.UpdateLocalBranchToRemoteParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
		}
		return err
	}

	dirty, err := infra.IsDirty(infra.IsDirtyParams{WorktreePath: wt.Path})
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("%s worktree has uncommitted changes", params.Branch)
	}
	return infra.FastForwardBranch(infra.FastForwardParams{WorktreePath: wt.Path, Onto: domain.RemoteBranchPrefix + params.Branch})
}

// divergenceParams holds inputs for divergence.
type divergenceParams struct {
	ProjectDir string
	Local      []string
	Remote     []string
}

// divergence computes ahead/behind only for local branches that have a same-name
// origin counterpart, bounding the number of git calls to the overlap.
func divergence(params divergenceParams) map[string]domain.AheadBehind {
	remoteSet := make(map[string]struct{}, len(params.Remote))
	for _, r := range params.Remote {
		remoteSet[strings.TrimPrefix(r, domain.RemoteBranchPrefix)] = struct{}{}
	}

	result := make(map[string]domain.AheadBehind, len(params.Local))
	for _, b := range params.Local {
		if _, ok := remoteSet[b]; !ok {
			continue
		}
		ab, err := infra.AheadBehind(infra.AheadBehindParams{
			ProjectDir: params.ProjectDir,
			Local:      b,
			Remote:     domain.RemoteBranchPrefix + b,
		})
		if err != nil {
			continue
		}
		result[b] = ab
	}
	return result
}
