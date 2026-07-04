package clean

import (
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// precheckClean runs the safety check for an explicit branch on the interactive
// path, reporting the idempotent absent/parent outcomes. handled=true means the
// command already produced its output and should return.
func precheckClean(opts *CleanOptions) (domain.CleanCheckResult, bool, error) {
	params := cleanParamsFor(opts, opts.Branch, opts.Force)
	var check domain.CleanCheckResult
	err := opts.Progress.Run(progress.RunParams{
		Message: "Checking worktree…",
		Animate: rules.IsHumanFormat(opts.Output),
	}, func() error {
		var e error
		check, e = worktree.Check(params)
		return e
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		output.Frame(opts.IO.Out, func() {
			output.Message(opts.IO.Out, fmt.Sprintf("Worktree %s already absent — nothing to clean", opts.Branch))
		})
		return domain.CleanCheckResult{}, true, nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Frame(opts.IO.ErrOut, func() {
			output.Warning(opts.IO.ErrOut, "Cannot clean the parent worktree.")
		})
		return domain.CleanCheckResult{}, true, nil
	}
	if err != nil {
		return domain.CleanCheckResult{}, false, err
	}
	return check, false, nil
}

// ensureSafeToClean is the --yes safety gate: skip the prompt but keep the checks,
// refusing a dirty / unpushed / open-PR worktree and directing the user to --force.
// Absent / parent worktrees are left to doClean, which handles both idempotently.
func ensureSafeToClean(opts *CleanOptions, params domain.CleanParams) error {
	var check domain.CleanCheckResult
	err := opts.Progress.Run(progress.RunParams{
		Message: "Checking worktree…",
		Animate: rules.IsHumanFormat(opts.Output),
	}, func() error {
		var e error
		check, e = worktree.Check(params)
		return e
	})
	if err != nil {
		// Absent / parent / other errors are handled by doClean; don't block here.
		return nil
	}
	if reason := rules.CleanUnsafeReason(check); reason != "" {
		return fmt.Errorf("worktree %s %s; pass --force to remove it anyway", params.Branch, reason)
	}
	return nil
}

// doClean performs the removal and reports the outcome. It is idempotent: cleaning
// an absent worktree is a success (agents can retry), and the parent worktree is
// refused with a warning.
func doClean(opts *CleanOptions, params domain.CleanParams, reparentPlan domain.CleanReparentPlan, applyReparent bool) error {
	wtPath := ""
	if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		wtPath = wt.Path
	}

	// Decide before removal (paths must still exist to resolve symlinks).
	cwd, _ := os.Getwd()
	insideRemoved := wtPath != "" && cwd != "" && rules.IsPathWithin(shared.ResolveSymlinks(wtPath), shared.ResolveSymlinks(cwd))

	shared.StopWorktreeServices(shared.StopWorktreeServicesParams{
		ErrOut:     opts.IO.ErrOut,
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	err := opts.Progress.Run(progress.RunParams{
		Message: "Cleaning worktree…",
		Animate: rules.IsHumanFormat(opts.Output),
	}, func() error {
		return worktree.Clean(params)
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		if opts.Output == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(opts.IO.Out, output.WriteWorktreeCleanJSONParams{
				Branch:        params.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Frame(opts.IO.Out, func() {
			output.Message(opts.IO.Out, fmt.Sprintf("Worktree %s already absent — nothing to clean", params.Branch))
		})
		return nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Frame(opts.IO.ErrOut, func() {
			output.Warning(opts.IO.ErrOut, "Cannot clean the parent worktree.")
		})
		return nil
	}
	if err != nil {
		return err
	}

	if insideRemoved {
		shared.RedirectToBase(params.ProjectDir)
	}

	reparented, reparentErr := applyChildReparent(params.StateDir, reparentPlan, applyReparent)
	if reparentErr != nil {
		return reparentErr
	}

	if opts.Output == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(opts.IO.Out, output.WriteWorktreeCleanJSONParams{
			Branch:           params.Branch,
			Path:             wtPath,
			Reparented:       reparented,
			OrphanedChildren: rules.CleanOrphanedChildren(reparentPlan, applyReparent),
		})
	}

	output.Frame(opts.IO.Out, func() {
		output.Success(opts.IO.Out, fmt.Sprintf("Cleaned worktree and branch %s", params.Branch))
		for _, child := range reparented {
			output.Success(opts.IO.Out, fmt.Sprintf("Reparented %s onto %s", child.Branch, child.NewParent))
		}
		for _, child := range rules.CleanOrphanedChildren(reparentPlan, applyReparent) {
			output.Warning(opts.IO.Out, fmt.Sprintf("%s still points at the removed parent %s — reparent it with `wtm reparent`", child.Branch, child.OldParent))
		}
	})
	return nil
}

// applyChildReparent reparents the orphaned children when authorized, otherwise
// returns nil so the caller can report them as still-orphaned.
func applyChildReparent(stateDir string, plan domain.CleanReparentPlan, apply bool) ([]domain.ReparentResult, error) {
	if !apply || len(plan.Children) == 0 {
		return nil, nil
	}
	return worktree.ApplyReparentChildren(worktree.ApplyReparentChildrenParams{Plan: plan, StateDir: stateDir})
}

