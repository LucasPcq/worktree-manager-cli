package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Reparent changes the recorded parent (source_branch) of a worktree. It only
// rewrites metadata — the actual rebase happens on the next `wtm sync`. The new
// parent must exist as a local branch or an origin remote-tracking branch
// (origin/x), and must keep the parent graph acyclic.
func Reparent(params domain.ReparentParams) (domain.ReparentResult, error) {
	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.ReparentResult{}, err
	}

	if !isManaged(params.StateDir, params.Branch) {
		return domain.ReparentResult{}, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, params.Branch)
	}

	if !infra.BranchOrRemoteExists(infra.BranchOrRemoteExistsParams{
		ProjectDir: params.ProjectDir,
		Ref:        params.NewParent,
	}) {
		return domain.ReparentResult{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, params.NewParent)
	}

	if err := rules.ValidateReparent(rules.ValidateReparentParams{
		Nodes:      nodes,
		Branch:     params.Branch,
		NewParent:  params.NewParent,
		BaseBranch: params.BaseBranch,
	}); err != nil {
		return domain.ReparentResult{}, err
	}

	return setSourceBranch(setSourceBranchParams{
		StateDir:  params.StateDir,
		Branch:    params.Branch,
		NewParent: params.NewParent,
	})
}

// PlanCleanReparent computes which children would be orphaned by cleaning a
// worktree and the grandparent they would move onto. It is best-effort: any error
// reading the worktree graph yields an empty plan (nothing to propose).
func PlanCleanReparent(params domain.CleanParams) domain.CleanReparentPlan {
	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.CleanReparentPlan{Branch: params.Branch}
	}

	grandparent := loadSourceBranch(params.StateDir, params.Branch)
	if grandparent == "" {
		grandparent = params.BaseBranch
	}

	children := rules.ChildrenOf(rules.ChildrenOfParams{Nodes: nodes, Branch: params.Branch})
	reparented := make([]domain.ReparentResult, 0, len(children))
	for _, child := range children {
		reparented = append(reparented, domain.ReparentResult{
			Branch:    child.Branch,
			OldParent: params.Branch,
			NewParent: grandparent,
		})
	}

	return domain.CleanReparentPlan{
		Branch:      params.Branch,
		Grandparent: grandparent,
		Children:    reparented,
	}
}

// ApplyReparentChildrenParams holds inputs for ApplyReparentChildren.
type ApplyReparentChildrenParams struct {
	Plan     domain.CleanReparentPlan
	StateDir string
}

// ApplyReparentChildren rewrites each child's metadata to point at the grandparent.
func ApplyReparentChildren(params ApplyReparentChildrenParams) ([]domain.ReparentResult, error) {
	applied := make([]domain.ReparentResult, 0, len(params.Plan.Children))
	for _, child := range params.Plan.Children {
		res, err := setSourceBranch(setSourceBranchParams{
			StateDir:  params.StateDir,
			Branch:    child.Branch,
			NewParent: params.Plan.Grandparent,
		})
		if err != nil {
			return applied, err
		}
		applied = append(applied, res)
	}
	return applied, nil
}

// ApplyReparentsParams holds inputs for ApplyReparents.
type ApplyReparentsParams struct {
	Reparents []domain.ReparentResult
	StateDir  string
}

// ApplyReparents rewrites each listed child's metadata to point at its NewParent.
// Unlike ApplyReparentChildren (a single grandparent), it applies a flat list of
// moves that may target different parents — used by prune, which reparents the
// children of several removed worktrees in one pass.
func ApplyReparents(params ApplyReparentsParams) ([]domain.ReparentResult, error) {
	applied := make([]domain.ReparentResult, 0, len(params.Reparents))
	for _, r := range params.Reparents {
		res, err := setSourceBranch(setSourceBranchParams{
			StateDir:  params.StateDir,
			Branch:    r.Branch,
			NewParent: r.NewParent,
		})
		if err != nil {
			return applied, err
		}
		applied = append(applied, res)
	}
	return applied, nil
}

// setSourceBranchParams holds inputs for setSourceBranch.
type setSourceBranchParams struct {
	StateDir  string
	Branch    string
	NewParent string
}

// setSourceBranch updates only the SourceBranch field of a worktree's metadata,
// preserving CreatedAt and EnvStrategy.
func setSourceBranch(params setSourceBranchParams) (domain.ReparentResult, error) {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return domain.ReparentResult{}, fmt.Errorf("read metadata for %s: %w", params.Branch, err)
	}

	old := meta.SourceBranch
	meta.SourceBranch = params.NewParent

	if err := writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), meta); err != nil {
		return domain.ReparentResult{}, err
	}

	return domain.ReparentResult{Branch: params.Branch, OldParent: old, NewParent: params.NewParent}, nil
}
