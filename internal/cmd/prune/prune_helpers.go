package prune

import (
	"os"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// scanPrune loads PR states (when needed) and plans the prune behind the scan
// spinner, then — since prune reads "done" from GitHub — warns once on stderr when
// the gh CLI is unavailable (never polluting JSON stdout).
func scanPrune(opts *PruneOptions, params domain.PruneParams) (domain.PrunePlan, error) {
	need := needPRs(opts)

	// Both gone-detection (git fetch --prune) and PR-detection (gh) hit the network,
	// so tell the user the spinner is fetching, not just scanning.
	scanMessage := "Scanning worktrees…"
	if need || (opts.Gone && !opts.NoFetch) {
		scanMessage = "Fetching remotes and scanning worktrees…"
	}

	var plan domain.PrunePlan
	var ghConn domain.GHConnection
	err := opts.Progress.Run(progress.RunParams{
		Message: scanMessage,
		Animate: rules.IsHumanFormat(opts.Output),
	}, func() error {
		var prs []domain.PRInfo
		if need {
			prs, ghConn = shared.LoadPRsAllStates(opts.cfg.ProjectDir)
		}
		var e error
		plan, e = worktree.PlanPrune(params, prs)
		return e
	})
	if err != nil {
		return domain.PrunePlan{}, err
	}

	if need {
		if title, lines, show := rules.PruneGHNotice(ghConn); show {
			output.Callout(opts.IO.ErrOut, title, lines)
		}
	}
	return plan, nil
}

// doPrune stops the worktrees' services, removes the selected worktrees behind the
// prune spinner, redirects the shell when the cwd was pruned, and reports the outcome.
func doPrune(opts *PruneOptions, params domain.PruneParams, plan domain.PrunePlan, orphaned []domain.ReparentResult) error {
	// Resolve now, before removal, whether the cwd sits inside a worktree we are
	// about to prune — the paths must still exist to canonicalize symlinks.
	dir, _ := os.Getwd()
	insidePruned := isInsidePruned(dir, plan.Selected)

	for _, c := range plan.Selected {
		shared.StopWorktreeServices(shared.StopWorktreeServicesParams{
			ErrOut:     opts.IO.ErrOut,
			ProjectDir: opts.cfg.ProjectDir,
			Branch:     c.Branch,
		})
	}

	var result domain.PruneResult
	err := opts.Progress.Run(progress.RunParams{
		Message: "Pruning worktrees…",
		Animate: rules.IsHumanFormat(opts.Output),
	}, func() error {
		var e error
		result, e = worktree.Prune(params, plan)
		return e
	})
	if err != nil {
		return err
	}
	result.Orphaned = orphaned

	// Like clean, if we removed the worktree the user was sitting in, ask the shell
	// wrapper to cd back to the base repo (no-op without shell integration).
	if insidePruned {
		shared.RedirectToBase(opts.cfg.ProjectDir)
	}

	if opts.Output == domain.OutputJSON {
		return output.WritePruneResultJSON(opts.IO.Out, result)
	}
	output.Frame(opts.IO.Out, func() {
		output.FormatPruneResult(opts.IO.Out, result)
	})
	return nil
}

// decidePruneReparentParams holds inputs for decidePruneReparent.
type decidePruneReparentParams struct {
	Reparents        []domain.ReparentResult
	ReparentChildren bool
	// PickerAnswer is the reparent choice the interactive picker already collected
	// (nil when the picker did not run or did not ask).
	PickerAnswer *bool
}

// decidePruneReparent resolves whether surviving children are reparented onto their
// grandparent. It returns the children to leave orphaned (empty when reparented).
// Precedence: the --reparent-children flag forces reparenting; otherwise the picker's
// in-wizard answer decides; non-interactively without either, children are orphaned.
func decidePruneReparent(params decidePruneReparentParams) (orphaned []domain.ReparentResult) {
	if len(params.Reparents) == 0 {
		return nil
	}
	if params.ReparentChildren {
		return nil
	}
	if params.PickerAnswer != nil {
		if *params.PickerAnswer {
			return nil
		}
		return params.Reparents
	}
	return params.Reparents
}

// isInsidePruned reports whether currentDir sits within any worktree about to be
// pruned, so the command can redirect the shell afterwards. Symlinks are resolved
// (the paths still exist at call time) to match a symlinked worktree path.
func isInsidePruned(currentDir string, selected []domain.PruneCandidate) bool {
	if currentDir == "" {
		return false
	}
	cwd := shared.ResolveSymlinks(currentDir)
	for _, c := range selected {
		if c.Path == "" {
			continue
		}
		if rules.IsPathWithin(shared.ResolveSymlinks(c.Path), cwd) {
			return true
		}
	}
	return false
}

// renderNothingToPrune reports an empty selection in the requested format.
func renderNothingToPrune(opts *PruneOptions) error {
	if opts.Output == domain.OutputJSON {
		return output.WritePruneResultJSON(opts.IO.Out, domain.PruneResult{})
	}
	output.Frame(opts.IO.Out, func() {
		output.Message(opts.IO.Out, "Nothing to prune.")
	})
	return nil
}

// renderPruneDryRun emits the preview plan without removing anything.
func renderPruneDryRun(opts *PruneOptions, plan domain.PrunePlan) error {
	if opts.Output == domain.OutputJSON {
		return output.WritePruneResultJSON(opts.IO.Out, domain.PruneResult{
			Pruned:     plan.Selected,
			Reparented: plan.Reparents,
			Skipped:    plan.Skipped,
			DryRun:     true,
		})
	}
	output.Frame(opts.IO.Out, func() {
		output.FormatPrunePlan(opts.IO.Out, plan)
	})
	return nil
}

// renderPruneAborted reports an aborted interactive prune.
func renderPruneAborted(opts *PruneOptions) error {
	output.Frame(opts.IO.Out, func() {
		output.Message(opts.IO.Out, "Aborted.")
	})
	return nil
}
