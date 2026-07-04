package clean

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/cmd/clean/wizard"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
)

// cleanRun dispatches to one of the three modes. The "Confirm phase" of the
// lifecycle is not a single call site for a composite-wizard command: dry-run
// short-circuits it, the interactive wizard performs its own confirmation, and the
// non-interactive path routes through the shared cmdutil.Confirm gate.
func cleanRun(opts *CleanOptions) error {
	if opts.DryRun {
		return cleanDryRun(opts)
	}
	if opts.Interactive {
		return cleanInteractive(opts)
	}
	return cleanNonInteractive(opts)
}

// cleanInteractive drives the composite wizard (picker → reparent → delete recap).
// For an explicit branch the safety check is pre-flighted so an absent/parent
// worktree is reported without a wizard; for a picked branch the wizard runs the
// check asynchronously via the injected closure.
func cleanInteractive(opts *CleanOptions) error {
	var preCheck *domain.CleanCheckResult
	if opts.Branch != "" {
		check, handled, err := precheckClean(opts)
		if err != nil || handled {
			return err
		}
		preCheck = &check
	}

	choice, err := opts.Wizard.Run(wizard.CleanPrompt{
		ProjectDir:        opts.cfg.ProjectDir,
		PreselectedBranch: opts.Branch,
		PreCheck:          preCheck,
		Force:             opts.Force,
		ReparentChildren:  opts.ReparentChildren,
		Check: func(branch string) domain.CleanCheckResult {
			check, checkErr := worktree.Check(cleanParamsFor(opts, branch, false))
			if checkErr != nil {
				return domain.CleanCheckResult{Branch: branch}
			}
			return check
		},
		ReparentPreview: func(branch string) domain.CleanReparentPlan {
			return worktree.PlanCleanReparent(cleanParamsFor(opts, branch, false))
		},
	})
	if err != nil {
		return err
	}
	if choice.Aborted {
		output.Frame(opts.IO.Out, func() {
			output.Message(opts.IO.Out, "Aborted.")
		})
		return nil
	}

	params := cleanParamsFor(opts, choice.Branch, choice.Force)
	reparentPlan := worktree.PlanCleanReparent(params)
	applyReparent := len(reparentPlan.Children) > 0 && choice.ReparentAsked && choice.ReparentChildren
	return doClean(opts, params, reparentPlan, applyReparent)
}

// cleanNonInteractive is reached only when !Interactive && !DryRun (i.e. --yes or a
// piped run). The confirmation gate is the single site enforcing "never destroy
// silently": here CanPrompt() is always false, so cmdutil.Confirm returns nil under
// --yes or the usage error when piped without --yes.
func cleanNonInteractive(opts *CleanOptions) error {
	params := cleanParamsFor(opts, opts.Branch, opts.Force)

	if err := cmdutil.Confirm(cmdutil.ConfirmParams{
		IO:       opts.IO,
		Prompter: opts.Prompter,
		Yes:      opts.Yes,
		Prompt:   fmt.Sprintf("Delete worktree and branch %s?", opts.Branch),
	}); err != nil {
		return err
	}

	reparentPlan := worktree.PlanCleanReparent(params)

	if opts.Yes && !opts.Force {
		if err := ensureSafeToClean(opts, params); err != nil {
			return err
		}
	}

	// canPrompt is false here, so the reparent decision resolves from the flag or
	// the safe default (leave orphaned) — never a prompt.
	applyReparent := rules.DecideCleanReparent(reparentPlan, opts.ReparentChildren)
	return doClean(opts, params, reparentPlan, applyReparent)
}

// cleanDryRun previews the plan and returns without mutating anything. It needs no
// confirmation (--yes) and works without a terminal.
func cleanDryRun(opts *CleanOptions) error {
	params := cleanParamsFor(opts, opts.Branch, opts.Force)

	wtPath := ""
	if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		wtPath = wt.Path
	}

	reparentPlan := worktree.PlanCleanReparent(params)
	applyReparent := rules.DecideCleanReparent(reparentPlan, opts.ReparentChildren)

	if opts.Output == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(opts.IO.Out, output.WriteWorktreeCleanJSONParams{
			Branch:           params.Branch,
			Path:             wtPath,
			DryRun:           true,
			Reparented:       rules.CleanReparentPreview(reparentPlan, applyReparent),
			OrphanedChildren: rules.CleanOrphanedChildren(reparentPlan, applyReparent),
		})
	}

	output.Frame(opts.IO.Out, func() {
		if wtPath != "" {
			output.Message(opts.IO.Out, fmt.Sprintf("Would remove worktree %s and branch %s", wtPath, params.Branch))
		} else {
			output.Message(opts.IO.Out, fmt.Sprintf("Would remove branch %s", params.Branch))
		}
		for _, child := range rules.CleanReparentPreview(reparentPlan, applyReparent) {
			output.Message(opts.IO.Out, fmt.Sprintf("Would reparent %s onto %s", child.Branch, child.NewParent))
		}
		for _, child := range rules.CleanOrphanedChildren(reparentPlan, applyReparent) {
			output.Message(opts.IO.Out, fmt.Sprintf("Would leave %s orphaned (points at removed parent %s)", child.Branch, child.OldParent))
		}
		output.Message(opts.IO.Out, "(dry-run: no changes made)")
	})
	return nil
}

// cleanParamsFor assembles the service params for a branch. The metier receives an
// already-resolved baseBranch, never the resolution logic.
func cleanParamsFor(opts *CleanOptions, branch string, force bool) domain.CleanParams {
	return domain.CleanParams{
		ProjectDir: opts.cfg.ProjectDir,
		StateDir:   opts.cfg.StateDir,
		Branch:     branch,
		Force:      force,
		BaseBranch: opts.baseBranch,
		Config:     opts.cfg.Config,
	}
}
