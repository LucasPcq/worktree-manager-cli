package worktree

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// SyncParams holds inputs for planning and running a cascade sync.
type SyncParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	BaseBranch string
	DryRun     bool
	// KeepConflict leaves a conflicting rebase in progress in its worktree (for
	// manual resolution) instead of aborting it. Descendants are still skipped;
	// independent branches keep syncing, so several worktrees may be left
	// mid-rebase.
	KeepConflict bool
	// SelectedBranches restricts the cascade to the named worktrees (topological
	// order is preserved). An empty slice means every managed worktree.
	SelectedBranches []string
}

// PlanSync builds the ordered cascade plan without touching any branch. Used to
// preview the operation and surface ordering errors (e.g. cycles) before acting.
func PlanSync(params SyncParams) (domain.SyncPlan, error) {
	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.SyncPlan{}, err
	}
	plan, err := rules.BuildSyncPlan(rules.BuildSyncPlanParams{
		Nodes:      nodes,
		BaseBranch: params.BaseBranch,
	})
	if err != nil {
		return domain.SyncPlan{}, err
	}
	plan.Steps = rules.FilterSyncSteps(plan.Steps, params.SelectedBranches)
	return plan, nil
}

// Sync runs the cascade: it fetches and fast-forwards the base, then rebases
// every managed worktree onto its (refreshed) parent in topological order. The
// rebases are entirely local — pushing is a separate step (see PushSynced).
func Sync(params SyncParams) (domain.SyncResult, error) {
	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.SyncResult{}, err
	}
	plan, err := rules.BuildSyncPlan(rules.BuildSyncPlanParams{
		Nodes:      nodes,
		BaseBranch: params.BaseBranch,
	})
	if err != nil {
		return domain.SyncResult{}, err
	}
	plan.Steps = rules.FilterSyncSteps(plan.Steps, params.SelectedBranches)

	mainPath := mainWorktreePath(nodes, params.ProjectDir)
	oldTips := captureTips(captureTipsParams{
		MainPath:   mainPath,
		BaseBranch: params.BaseBranch,
		Steps:      plan.Steps,
	})

	result := domain.SyncResult{
		BaseBranch:       params.BaseBranch,
		BaseOldTip:       oldTips[params.BaseBranch],
		BaseNewTip:       oldTips[params.BaseBranch],
		SelectedBranches: stepBranches(plan.Steps),
	}

	if !params.DryRun {
		newTip, updated := updateBase(updateBaseParams{
			MainPath:   mainPath,
			BaseBranch: params.BaseBranch,
		})
		result.BaseUpdated = updated
		result.BaseNewTip = newTip
	}

	skipped := make(map[string]bool)
	for _, step := range plan.Steps {
		stepResult, blocks := evaluateStep(stepEval{
			Step:         step,
			OldTips:      oldTips,
			Skipped:      skipped,
			DryRun:       params.DryRun,
			KeepConflict: params.KeepConflict,
		})
		if blocks {
			skipped[step.Branch] = true
		}
		result.Steps = append(result.Steps, stepResult)
	}

	return result, nil
}

// stepBranches lists the branches covered by a cascade's steps — the explicit
// selection or, for --all, every managed worktree. Always non-nil so the JSON
// field marshals as [] rather than null.
func stepBranches(steps []domain.SyncStep) []string {
	branches := make([]string, 0, len(steps))
	for _, step := range steps {
		branches = append(branches, step.Branch)
	}
	return branches
}

// PushSyncedParams holds inputs for pushing the successfully-rebased branches.
type PushSyncedParams struct {
	ProjectDir string
	Result     domain.SyncResult
}

// PushSynced force-pushes (with lease) every branch that was rebased in the run.
// It mutates and returns a copy of the result with Pushed flags set.
func PushSynced(params PushSyncedParams) domain.SyncResult {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return params.Result
	}

	pathByBranch := make(map[string]string, len(worktrees))
	for _, w := range worktrees {
		pathByBranch[w.Branch] = w.Path
	}

	for i := range params.Result.Steps {
		step := &params.Result.Steps[i]
		if !step.PushPending || step.Pushed {
			continue
		}
		path := pathByBranch[step.Branch]
		if path == "" {
			continue
		}
		if pushErr := infra.PushForceWithLease(infra.PushForceParams{
			WorktreePath: path,
			Branch:       step.Branch,
		}); pushErr != nil {
			step.Detail = pushErr.Error()
			continue
		}
		step.Pushed = true
	}

	return params.Result
}

type NodesParams struct {
	ProjectDir string
	StateDir   string
}

// Nodes is the managed worktrees with their recorded parents: the graph the
// reparent and sync rules are validated against. flow/ cannot reach infra/, so
// this is where it reads it from.
func Nodes(params NodesParams) ([]domain.WorktreeNode, error) {
	return buildNodes(params.ProjectDir, params.StateDir)
}

func buildNodes(projectDir, stateDir string) ([]domain.WorktreeNode, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil, err
	}

	nodes := make([]domain.WorktreeNode, 0, len(worktrees))
	for _, w := range worktrees {
		source := ""
		if !w.IsMain {
			source = loadSourceBranch(stateDir, w.Branch)
		}
		nodes = append(nodes, domain.WorktreeNode{
			Branch:           w.Branch,
			Path:             w.Path,
			SourceBranch:     source,
			IsMain:           w.IsMain,
			RebaseInProgress: w.RebaseInProgress,
		})
	}

	return nodes, nil
}

func mainWorktreePath(nodes []domain.WorktreeNode, projectDir string) string {
	for _, node := range nodes {
		if node.IsMain {
			return node.Path
		}
	}
	return projectDir
}

type captureTipsParams struct {
	MainPath   string
	BaseBranch string
	Steps      []domain.SyncStep
}

// captureTips records every branch tip BEFORE any rebase. The recorded parent
// tip is later used as the rebase upstream so only a child's own commits replay.
func captureTips(params captureTipsParams) map[string]string {
	tips := make(map[string]string, len(params.Steps)+1)

	if tip, err := infra.Tip(infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	}); err == nil {
		tips[params.BaseBranch] = tip
	}

	for _, step := range params.Steps {
		if tip, err := infra.Tip(infra.TipParams{
			WorktreePath: step.Path,
			Ref:          step.Branch,
		}); err == nil {
			tips[step.Branch] = tip
		}
	}

	return tips
}

type updateBaseParams struct {
	MainPath   string
	BaseBranch string
}

// updateBase fetches origin and fast-forwards the local base branch. A diverged
// base (no fast-forward), missing remote, or a dirty main worktree is non-fatal:
// the cascade then rebases onto the local base as-is. The dirty guard mirrors the
// per-worktree behaviour — the base is only advanced when its worktree is clean.
func updateBase(params updateBaseParams) (string, bool) {
	currentTip, _ := infra.Tip(infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	})

	if dirty, err := infra.IsDirty(infra.IsDirtyParams{WorktreePath: params.MainPath}); err != nil || dirty {
		return currentTip, false
	}

	if fetchErr := infra.FetchBranch(infra.FetchBranchParams{
		ProjectDir: params.MainPath,
		Branch:     params.BaseBranch,
	}); fetchErr != nil {
		return currentTip, false
	}

	if ffErr := infra.FastForwardBranch(infra.FastForwardParams{
		WorktreePath: params.MainPath,
		Onto:         "origin/" + params.BaseBranch,
	}); ffErr != nil {
		return currentTip, false
	}

	newTip, _ := infra.Tip(infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	})
	return newTip, newTip != currentTip
}

type integrateOutcome int

const (
	integrateProceed integrateOutcome = iota
	integrateDiverged
)

// integrateRemote fast-forwards a branch from its own origin/<branch> before it
// is rebased. Returns integrateDiverged when local and remote have both moved
// (no fast-forward possible). A missing remote branch is a no-op.
func integrateRemote(step domain.SyncStep) integrateOutcome {
	if fetchErr := infra.FetchBranch(infra.FetchBranchParams{
		ProjectDir: step.Path,
		Branch:     step.Branch,
	}); fetchErr != nil {
		return integrateProceed
	}

	remoteRef := "origin/" + step.Branch
	localHasRemote := infra.IsAncestor(infra.IsAncestorParams{
		WorktreePath: step.Path,
		Ancestor:     remoteRef,
		Descendant:   step.Branch,
	})
	if localHasRemote {
		return integrateProceed
	}

	remoteHasLocal := infra.IsAncestor(infra.IsAncestorParams{
		WorktreePath: step.Path,
		Ancestor:     step.Branch,
		Descendant:   remoteRef,
	})
	if remoteHasLocal {
		infra.FastForwardBranch(infra.FastForwardParams{
			WorktreePath: step.Path,
			Onto:         remoteRef,
		})
		return integrateProceed
	}

	// Neither ref is an ancestor of the other. This is a genuine divergence only
	// when the remote carries commits not already integrated (by patch) into the
	// local branch. After a prior local rebase the remote's commits are
	// patch-present locally (just rewritten), so there is nothing to reconcile —
	// proceed and let the rebase see behind == 0 (up_to_date).
	if infra.RemoteHasUnintegratedCommits(infra.RemoteBranchParams{
		WorktreePath: step.Path,
		Branch:       step.Branch,
	}) {
		return integrateDiverged
	}
	return integrateProceed
}

type stepEval struct {
	Step         domain.SyncStep
	OldTips      map[string]string
	Skipped      map[string]bool
	DryRun       bool
	KeepConflict bool
}

// evaluateStep computes the outcome of one cascade step. The second return value
// reports whether this branch must block its descendants (skip/conflict/error).
func evaluateStep(e stepEval) (domain.SyncStepResult, bool) {
	result := domain.SyncStepResult{
		Branch:       e.Step.Branch,
		SourceBranch: e.Step.SourceBranch,
		Path:         e.Step.Path,
		OldTip:       e.OldTips[e.Step.Branch],
		NewTip:       e.OldTips[e.Step.Branch],
	}

	if e.Step.SourceBranch == "" {
		result.Status = domain.SyncStatusUnknownParent
		result.Detail = "no recorded parent branch"
		return result, true
	}

	if e.Skipped[e.Step.SourceBranch] {
		result.Status = domain.SyncStatusSkippedAncestor
		result.Detail = "ancestor " + e.Step.SourceBranch + " was not synced"
		return result, true
	}

	// A worktree with a rebase already paused mid-way (e.g. a prior keep-conflict
	// run) cannot be rebased again until it is resolved. Report it distinctly rather
	// than as a generic "uncommitted changes" skip.
	if e.Step.RebaseInProgress {
		result.Status = domain.SyncStatusRebaseInProgress
		result.Detail = "rebase already in progress — finish it with git rebase --continue (or --abort)"
		return result, true
	}

	dirty, dirtyErr := infra.IsDirty(infra.IsDirtyParams{WorktreePath: e.Step.Path})
	if dirtyErr != nil {
		// Could not determine cleanliness — do not rebase blind. Block the
		// branch (and its descendants) so a real git failure is surfaced rather
		// than silently treated as a clean worktree.
		result.Status = domain.SyncStatusError
		result.Detail = "cannot check working tree: " + dirtyErr.Error()
		return result, true
	}
	if dirty {
		result.Status = domain.SyncStatusSkippedDirty
		result.Detail = "uncommitted changes"
		return result, true
	}

	// Pull the branch's own remote (fast-forward only) before rebasing, so
	// commits merged into origin/<branch> elsewhere are included. A true
	// committed divergence (no fast-forward) is left for the user to reconcile.
	// Skipped in dry-run, which stays fully offline.
	if !e.DryRun && integrateRemote(e.Step) == integrateDiverged {
		result.Status = domain.SyncStatusDiverged
		result.Detail = "local and origin/" + e.Step.Branch + " have diverged"
		return result, true
	}

	if tip, err := infra.Tip(infra.TipParams{
		WorktreePath: e.Step.Path,
		Ref:          e.Step.SourceBranch,
	}); err == nil {
		result.OntoTip = tip
	}

	behind, behindErr := infra.Behind(infra.BehindParams{
		WorktreePath: e.Step.Path,
		Branch:       e.Step.Branch,
		Upstream:     e.Step.SourceBranch,
	})
	if behindErr != nil {
		// A failed behind-count must not be read as "0 → up to date", which
		// would skip the rebase and report a stale branch as synced.
		result.Status = domain.SyncStatusError
		result.Detail = behindErr.Error()
		return result, true
	}
	if behind == 0 {
		result.Status = domain.SyncStatusUpToDate
		if tip, err := infra.Tip(infra.TipParams{
			WorktreePath: e.Step.Path,
			Ref:          e.Step.Branch,
		}); err == nil {
			result.NewTip = tip
		}
		if !e.DryRun {
			result.PushPending = infra.AheadOfRemote(infra.RemoteBranchParams{
				WorktreePath: e.Step.Path,
				Branch:       e.Step.Branch,
			})
		}
		return result, false
	}

	upstream := e.OldTips[e.Step.SourceBranch]
	if upstream == "" {
		upstream = e.Step.SourceBranch
	}
	result.CommitsReplayed, _ = infra.CommitCount(infra.CommitCountParams{
		WorktreePath: e.Step.Path,
		Range:        upstream + ".." + e.Step.Branch,
	})

	if e.DryRun {
		result.Status = domain.SyncStatusSynced
		result.PushPending = true
		return result, false
	}

	rebaseResult, err := infra.RebaseOnto(infra.RebaseOntoParams{
		WorktreePath: e.Step.Path,
		NewBase:      e.Step.SourceBranch,
		Upstream:     upstream,
		Branch:       e.Step.Branch,
		KeepConflict: e.KeepConflict,
	})
	if err != nil {
		result.Status = domain.SyncStatusError
		result.Detail = err.Error()
		return result, true
	}
	if rebaseResult.Conflicted {
		result.Status = domain.SyncStatusConflict
		result.ConflictFiles = rebaseResult.Files
		result.KeptInProgress = rebaseResult.Kept
		if rebaseResult.Kept {
			result.Detail = "rebase conflict (left in progress)"
		} else {
			result.Detail = "rebase conflict (aborted, working tree clean)"
		}
		return result, true
	}

	result.Status = domain.SyncStatusSynced
	if tip, tipErr := infra.Tip(infra.TipParams{
		WorktreePath: e.Step.Path,
		Ref:          e.Step.Branch,
	}); tipErr == nil {
		result.NewTip = tip
	}
	result.PushPending = infra.AheadOfRemote(infra.RemoteBranchParams{
		WorktreePath: e.Step.Path,
		Branch:       e.Step.Branch,
	})
	return result, false
}
