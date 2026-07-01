package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	prunetui "github.com/LucasPcq/wtm/internal/tui/prune"
)

// newPruneCmd creates the wtm prune subcommand.
func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPrune,
		Short: "Remove finished worktrees (merged, closed PR, gone, or old) in one pass",
		Long: "Batch-remove worktrees whose work is done, reparenting any surviving children onto\n" +
			"their grandparent (like `clean --reparent-children`). By default prune considers\n" +
			"every finished worktree: merged into the base, closed/merged PR, or upstream branch\n" +
			"gone. The reason flags restrict to specific categories — --merged (no commits ahead),\n" +
			"--closed (PR merged/closed, needs gh), --gone (remote branch deleted).\n" +
			"\n" +
			"gone-detection runs `git fetch --prune` first so deleted remote branches are seen\n" +
			"(pass --no-fetch to skip). --merged does not catch squash-merges (the branch keeps\n" +
			"distinct commits); --gone or --closed do.\n" +
			"\n" +
			"On a TTY, matches are shown for review (unsafe ones unchecked), then a prune\n" +
			"confirmation, then — like clean — a dedicated confirmation to reparent surviving\n" +
			"children onto their grandparent (or leave them orphaned). The main worktree and base\n" +
			"branch are always protected; the current worktree is removed and the shell\n" +
			"redirected to the base repo. Like clean, worktrees that are dirty, have unpushed\n" +
			"commits, or have an open PR are unsafe and need --force. Use --yes to skip the\n" +
			"prompts (required with --output json); non-interactively, children are left orphaned\n" +
			"unless --reparent-children is passed. --dry-run previews without changing anything.",
		Args: cobra.NoArgs,
		RunE: runPrune,
	}

	cmd.Flags().Bool(domain.FlagMerged, false, "Restrict to worktrees whose branch is merged into the base (no commits ahead)")
	cmd.Flags().Bool(domain.FlagClosed, false, "Restrict to worktrees whose PR is merged or closed (needs gh)")
	cmd.Flags().Bool(domain.FlagGone, false, "Restrict to worktrees whose upstream branch was deleted on the remote")
	cmd.Flags().Bool(domain.FlagNoFetch, false, "Skip the git fetch --prune that gone-detection performs; use already-fetched state")
	cmd.Flags().Bool(domain.FlagForce, false, "Also remove unsafe worktrees (dirty, unpushed commits, or open PR)")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, "Reparent orphaned child worktrees onto the grandparent (non-interactive; otherwise you're asked)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation/selection prompt (keeps every match)")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview what would be pruned without removing anything")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runPrune(cmd *cobra.Command, _ []string) error {
	merged, _ := cmd.Flags().GetBool(domain.FlagMerged)
	closed, _ := cmd.Flags().GetBool(domain.FlagClosed)
	gone, _ := cmd.Flags().GetBool(domain.FlagGone)
	noFetch, _ := cmd.Flags().GetBool(domain.FlagNoFetch)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	reparentChildren, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	// Default is broad: with no reason flag, consider every finished worktree
	// (merged, closed-PR, or gone). Reason flags narrow to specific categories.
	if !merged && !closed && !gone {
		merged, closed, gone = true, true, true
	}

	if format == domain.OutputJSON && !yes && !dryRun {
		return fmt.Errorf("--output json requires --yes or --dry-run (the selection prompt cannot run in JSON mode)")
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	baseBranch := resolveBase("", cfg)
	interactive := rules.IsHumanFormat(format)
	canPrompt := interactive && term.IsTerminal(int(os.Stdin.Fd()))
	interactivePick := canPrompt && !yes && !dryRun

	if !canPrompt && !yes && !dryRun {
		return fmt.Errorf("prune needs a terminal to confirm; pass --yes to run non-interactively")
	}

	// In the interactive picker, classify with force so dirty worktrees surface as
	// (unchecked) candidates the user can opt into; the confirm gates whether they
	// are actually removed. Non-interactive/dry-run honor the --force flag as-is.
	planForce := force || interactivePick

	params := domain.PruneParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Config:     cfg.Config,
		BaseBranch: baseBranch,
		Merged:     merged,
		Closed:     closed,
		Gone:       gone,
		NoFetch:    noFetch,
		Force:      planForce,
		DryRun:     dryRun,
	}

	// PR states are needed to match --closed candidates and to enforce the
	// open-PR safety guard (mirrors clean) whenever removal is not forced. With
	// --force the guard is bypassed, so PRs are only loaded for the closed filter.
	needPRs := closed || !force

	// Both gone-detection (git fetch --prune) and PR-detection (gh) hit the
	// network, so tell the user the spinner is fetching, not just scanning.
	scanMessage := "Scanning worktrees…"
	if needPRs || (gone && !noFetch) {
		scanMessage = "Fetching remotes and scanning worktrees…"
	}

	var plan domain.PrunePlan
	err = components.RunLoading(components.LoadingParams{
		Message: scanMessage,
		Animate: interactive,
		Work: func() error {
			var prs []domain.PRInfo
			if needPRs {
				prs = shared.LoadPRsAllStatesGraceful(cfg.ProjectDir)
			}
			var e error
			plan, e = worktree.PlanPrune(params, prs)
			return e
		},
	})
	if err != nil {
		return err
	}

	if len(plan.Selected) == 0 {
		return renderNothingToPrune(cmd, format)
	}

	if dryRun {
		return renderPruneDryRun(cmd, format, plan)
	}

	if interactivePick {
		res, pickErr := prunetui.Run(plan)
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return renderPruneAborted(cmd)
		}
		if pickErr != nil {
			return pickErr
		}
		plan = rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
			Plan:       plan,
			Chosen:     res.Branches,
			BaseBranch: baseBranch,
			Force:      res.Force,
		})
		if len(plan.Selected) == 0 {
			return renderNothingToPrune(cmd, format)
		}
	}

	// Reparenting surviving children rewrites their metadata, so — like clean — it
	// needs explicit consent: the --reparent-children flag, an interactive Yes, or
	// (non-interactive without the flag) it is skipped and they are left orphaned.
	orphaned, aborted := decidePruneReparent(cmd, decidePruneReparentParams{
		Reparents:        plan.Reparents,
		ReparentChildren: reparentChildren,
		Interactive:      interactivePick,
	})
	if aborted {
		return renderPruneAborted(cmd)
	}
	if len(orphaned) > 0 {
		plan.Reparents = nil
	}

	// Resolve now, before removal, whether the cwd sits inside a worktree we are
	// about to prune — the paths must still exist to canonicalize symlinks.
	insidePruned := isInsidePruned(dir, plan.Selected)

	for _, c := range plan.Selected {
		stopWorktreeServices(cmd, cfg.ProjectDir, c.Branch)
	}

	var result domain.PruneResult
	err = components.RunLoading(components.LoadingParams{
		Message: "Pruning worktrees…",
		Animate: interactive,
		Work: func() error {
			var e error
			result, e = worktree.Prune(params, plan)
			return e
		},
	})
	if err != nil {
		return err
	}
	result.Orphaned = orphaned

	// Like clean, if we removed the worktree the user was sitting in, ask the
	// shell wrapper to cd back to the base repo (no-op without shell integration).
	if insidePruned {
		redirectToBase(cfg.ProjectDir)
	}

	if format == domain.OutputJSON {
		return output.WritePruneResultJSON(cmd.OutOrStdout(), result)
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.FormatPruneResult(cmd.OutOrStdout(), result)
	})
	return nil
}

// decidePruneReparentParams holds inputs for decidePruneReparent.
type decidePruneReparentParams struct {
	Reparents        []domain.ReparentResult
	ReparentChildren bool
	Interactive      bool
}

// decidePruneReparent resolves whether surviving children are reparented onto
// their grandparent, mirroring clean's decideReparent. It returns the children to
// leave orphaned (empty when they are reparented) and whether the user aborted at
// the interactive confirmation. With no children it is a no-op. The flag is the
// explicit permission in non-interactive mode; interactively the user is asked.
func decidePruneReparent(cmd *cobra.Command, params decidePruneReparentParams) (orphaned []domain.ReparentResult, abort bool) {
	if len(params.Reparents) == 0 {
		return nil, false
	}
	if params.ReparentChildren {
		return nil, false
	}
	if !params.Interactive {
		return params.Reparents, false
	}
	apply, aborted := confirmPruneReparent(cmd, params.Reparents)
	if aborted {
		return nil, true
	}
	if apply {
		return nil, false
	}
	return params.Reparents, false
}

// confirmPruneReparent shows the proposed reparenting of pruned worktrees'
// orphaned children and asks for a dedicated confirmation, mirroring clean. Yes →
// reparent onto the grandparent; No → leave the children orphaned; Esc → abort
// the whole prune (nothing is removed).
func confirmPruneReparent(cmd *cobra.Command, moves []domain.ReparentResult) (apply bool, abort bool) {
	output.FormatPruneReparentProposal(cmd.ErrOrStderr(), moves)

	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("Reparent %d child worktree(s) onto their grandparent?", len(moves)),
		DefaultYes: true,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	if err != nil {
		return false, true
	}
	return confirmed, false
}

// renderPruneAborted reports an aborted interactive prune.
func renderPruneAborted(cmd *cobra.Command) error {
	output.Frame(cmd.OutOrStdout(), func() {
		output.Message(cmd.OutOrStdout(), "Aborted.")
	})
	return nil
}

// isInsidePruned reports whether currentDir sits within any worktree about to be
// pruned, so the command can redirect the shell afterwards. Symlinks are resolved
// (the paths still exist at call time) to match a symlinked worktree path.
func isInsidePruned(currentDir string, selected []domain.PruneCandidate) bool {
	if currentDir == "" {
		return false
	}
	cwd := resolveSymlinks(currentDir)
	for _, c := range selected {
		if c.Path == "" {
			continue
		}
		if rules.IsPathWithin(resolveSymlinks(c.Path), cwd) {
			return true
		}
	}
	return false
}

// renderNothingToPrune reports an empty selection in the requested format.
func renderNothingToPrune(cmd *cobra.Command, format string) error {
	if format == domain.OutputJSON {
		return output.WritePruneResultJSON(cmd.OutOrStdout(), domain.PruneResult{})
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.Message(cmd.OutOrStdout(), "Nothing to prune.")
	})
	return nil
}

// renderPruneDryRun emits the preview plan without removing anything.
func renderPruneDryRun(cmd *cobra.Command, format string, plan domain.PrunePlan) error {
	if format == domain.OutputJSON {
		return output.WritePruneResultJSON(cmd.OutOrStdout(), domain.PruneResult{
			Pruned:     plan.Selected,
			Reparented: plan.Reparents,
			Skipped:    plan.Skipped,
			DryRun:     true,
		})
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.FormatPrunePlan(cmd.OutOrStdout(), plan)
	})
	return nil
}
