package prune

import (
	"github.com/LucasPcq/wtm/internal/cmd/prune/wizard"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
)

// pruneRun dispatches to one of the three modes. The "Confirm phase" of the
// lifecycle is not a single call site: dry-run short-circuits it, the interactive
// picker performs its own confirmation, and the non-interactive path routes through
// the shared cmdutil.Confirm gate.
func pruneRun(opts *PruneOptions) error {
	if opts.DryRun {
		return pruneDryRun(opts)
	}
	if opts.Interactive {
		return pruneInteractive(opts)
	}
	return pruneNonInteractive(opts)
}

// pruneParamsFor assembles the service params. Force is planForce: classify with
// force in the interactive picker so unsafe worktrees surface (unchecked) as opt-in
// candidates the confirm re-gates; non-interactive/dry-run honor --force literally.
func pruneParamsFor(opts *PruneOptions) domain.PruneParams {
	planForce := opts.Force || (opts.Interactive && !opts.DryRun)
	return domain.PruneParams{
		ProjectDir: opts.cfg.ProjectDir,
		StateDir:   opts.cfg.StateDir,
		Config:     opts.cfg.Config,
		BaseBranch: opts.baseBranch,
		Merged:     opts.Merged,
		Closed:     opts.Closed,
		Gone:       opts.Gone,
		NoFetch:    opts.NoFetch,
		Force:      planForce,
		DryRun:     opts.DryRun,
	}
}

// needPRs reports whether PR states must be loaded: they are the source of truth for
// --merged/--closed, and enforce the open-PR safety guard whenever removal is not
// forced. With --force the guard is bypassed, so PRs load only for the filters then.
func needPRs(opts *PruneOptions) bool {
	return opts.Merged || opts.Closed || !opts.Force
}

// pruneDryRun previews the plan without mutating anything. It needs no confirmation
// (--yes) and works without a terminal.
func pruneDryRun(opts *PruneOptions) error {
	plan, err := scanPrune(opts, pruneParamsFor(opts))
	if err != nil {
		return err
	}
	if len(plan.Selected) == 0 {
		return renderNothingToPrune(opts)
	}
	return renderPruneDryRun(opts, plan)
}

// pruneInteractive drives the multi-select picker: scan, then let the user deselect
// candidates and decide reparenting, then apply. --force is threaded into the picker
// as a preset (PrunePrompt.Force) so it pre-lifts refusals without re-asking (like clean).
func pruneInteractive(opts *PruneOptions) error {
	params := pruneParamsFor(opts)
	plan, err := scanPrune(opts, params)
	if err != nil {
		return err
	}
	if len(plan.Selected) == 0 {
		return renderNothingToPrune(opts)
	}

	choice, err := opts.Wizard.Run(wizard.PrunePrompt{
		Plan:  plan,
		Force: opts.Force,
		ReparentPreview: func(chosen []string, force bool) []domain.ReparentResult {
			return rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
				Plan:       plan,
				Chosen:     chosen,
				BaseBranch: opts.baseBranch,
				Force:      force,
			}).Reparents
		},
	})
	if err != nil {
		return err
	}
	if choice.Aborted {
		return renderPruneAborted(opts)
	}

	force := opts.Force || choice.Force
	plan = rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
		Plan:       plan,
		Chosen:     choice.Branches,
		BaseBranch: opts.baseBranch,
		Force:      force,
	})
	if len(plan.Selected) == 0 {
		return renderNothingToPrune(opts)
	}

	// Reuse the picker's in-wizard reparent answer (when its step ran) instead of a
	// second, orphaned prompt.
	var pickerReparent *bool
	if choice.ReparentAsked {
		answer := choice.ReparentChildren
		pickerReparent = &answer
	}
	orphaned := decidePruneReparent(decidePruneReparentParams{
		Reparents:        plan.Reparents,
		ReparentChildren: opts.ReparentChildren,
		PickerAnswer:     pickerReparent,
	})
	if len(orphaned) > 0 {
		plan.Reparents = nil
	}
	return doPrune(opts, params, plan, orphaned)
}

// pruneNonInteractive is reached only when !Interactive && !DryRun (i.e. --yes or a
// piped run). cmdutil.Confirm is the single site enforcing "never destroy silently":
// CanPrompt() is false here, so it returns nil under --yes or a FlagError when piped.
func pruneNonInteractive(opts *PruneOptions) error {
	if err := cmdutil.Confirm(cmdutil.ConfirmParams{
		IO:       opts.IO,
		Prompter: opts.Prompter,
		Yes:      opts.Yes,
		Prompt:   "Prune the matching finished worktrees?",
	}); err != nil {
		return err
	}

	params := pruneParamsFor(opts)
	plan, err := scanPrune(opts, params)
	if err != nil {
		return err
	}
	if len(plan.Selected) == 0 {
		return renderNothingToPrune(opts)
	}

	// canPrompt is false here, so the reparent decision resolves from the flag or the
	// safe default (leave orphaned) — never a prompt.
	orphaned := decidePruneReparent(decidePruneReparentParams{
		Reparents:        plan.Reparents,
		ReparentChildren: opts.ReparentChildren,
		PickerAnswer:     nil,
	})
	if len(orphaned) > 0 {
		plan.Reparents = nil
	}
	return doPrune(opts, params, plan, orphaned)
}
