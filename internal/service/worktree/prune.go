package worktree

import (
	"errors"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// PlanPrune gathers the data ClassifyPrune needs — worktree statuses, the parent
// graph, PR states, and (for the Gone filter) a pruning fetch plus per-branch
// upstream-gone probing — then returns the side-effect-free plan. The PR set is
// passed in so the command owns its graceful/spinner behavior.
func PlanPrune(params domain.PruneParams, prs []domain.PRInfo) (domain.PrunePlan, error) {
	if params.Gone && !params.NoFetch {
		// Best-effort, like sync's base fetch: stale local refs are better than a
		// hard failure offline. The user opted into the network cost with --gone.
		_ = infra.FetchPrune(infra.FetchPruneParams{ProjectDir: params.ProjectDir})
	}

	statuses, err := List(domain.ListParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Config:     params.Config,
	})
	if err != nil {
		return domain.PrunePlan{}, err
	}

	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.PrunePlan{}, err
	}

	gone := map[string]bool{}
	if params.Gone {
		gone = computeGone(params.ProjectDir, statuses)
	}

	return rules.ClassifyPrune(rules.ClassifyPruneParams{
		Statuses:   statuses,
		Nodes:      nodes,
		PRStates:   prStates(prs),
		Gone:       gone,
		Merged:     params.Merged,
		Closed:     params.Closed,
		GoneFilter: params.Gone,
		BaseBranch: params.BaseBranch,
		Force:      params.Force,
	}), nil
}

// Prune removes the selected worktrees (reusing Clean, which deletes the local
// branch) and reparents the surviving children onto their grandparent. An absent
// worktree is tolerated so a re-run is idempotent. A dry run returns the plan as a
// result without touching anything.
func Prune(params domain.PruneParams, plan domain.PrunePlan) (domain.PruneResult, error) {
	result := domain.PruneResult{
		Pruned:     []domain.PruneCandidate{},
		Reparented: []domain.ReparentResult{},
		Skipped:    plan.Skipped,
		DryRun:     params.DryRun,
	}

	if params.DryRun {
		result.Pruned = plan.Selected
		result.Reparented = plan.Reparents
		return result, nil
	}

	for _, cand := range plan.Selected {
		err := Clean(domain.CleanParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Branch:     cand.Branch,
			Force:      true, // safety was already decided during classification
			BaseBranch: params.BaseBranch,
			Config:     params.Config,
		})
		if err != nil && !errors.Is(err, domain.ErrWorktreeNotFound) {
			return result, err
		}
		result.Pruned = append(result.Pruned, cand)
	}

	reparented, err := ApplyReparents(ApplyReparentsParams{
		Reparents: plan.Reparents,
		StateDir:  params.StateDir,
	})
	if err != nil {
		return result, err
	}
	if len(reparented) > 0 {
		result.Reparented = reparented
	}
	return result, nil
}

// prStates maps each branch to its normalized PR state, keeping the first hit
// (ListPRsAllStates returns newest-first).
func prStates(prs []domain.PRInfo) map[string]string {
	states := make(map[string]string, len(prs))
	for _, pr := range prs {
		if _, seen := states[pr.Branch]; seen {
			continue
		}
		states[pr.Branch] = pr.State
	}
	return states
}

// computeGone probes, concurrently, whether each worktree's branch has a deleted
// upstream ("[gone]"). Bounded by statusWorkers.
func computeGone(projectDir string, statuses []domain.WorktreeStatus) map[string]bool {
	result := make(map[string]bool, len(statuses))
	var mu sync.Mutex

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for _, st := range statuses {
		wg.Add(1)
		sem <- struct{}{}
		go func(branch string) {
			defer wg.Done()
			defer func() { <-sem }()
			if !infra.UpstreamGone(infra.UpstreamGoneParams{ProjectDir: projectDir, Branch: branch}) {
				return
			}
			mu.Lock()
			result[branch] = true
			mu.Unlock()
		}(st.Branch)
	}
	wg.Wait()

	return result
}
