package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	relocatetui "github.com/LucasPcq/wtm/internal/tui/relocate"
)

// newRelocateCmd creates the wtm relocate subcommand.
func newRelocateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRelocate,
		Short: "Move worktrees to align with base_path and adopt external ones",
		Long: "Reconcile every worktree with the configured base_path. Worktrees not under it are\n" +
			"moved (git worktree move) and worktrees created outside wtm are adopted (their parent\n" +
			"recorded so `wtm sync` works). Pass --to to change base_path and move existing worktrees\n" +
			"to the new location. Dirty or locked worktrees are skipped unless --force; an occupied\n" +
			"target path is never overwritten.",
		Args: cobra.NoArgs,
		RunE: runRelocate,
	}

	cmd.Flags().String(domain.FlagTo, "", "New base_path (relative to repo root); also moves existing worktrees there")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the plan without moving or adopting anything")
	cmd.Flags().Bool(domain.FlagForce, false, "Move dirty or locked worktrees too")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation and parent prompts (parents default to the base branch)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runRelocate(cmd *cobra.Command, _ []string) error {
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if err := rules.ValidateRelocateTarget(to); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	params := worktree.RelocateParams{
		ProjectDir:     cfg.ProjectDir,
		StateDir:       cfg.StateDir,
		Config:         cfg.Config,
		TargetBasePath: resolveTargetBasePath(to, cfg),
		BaseBranch:     resolveBase("", cfg),
		Force:          force,
		DryRun:         dryRun,
	}

	interactive := rules.IsHumanFormat(format)

	// Opt-in base_path edit. Done before planning so the plan, its recap, and the
	// empty check all reflect the chosen base_path. Skipped when --to already fixed
	// it, when --yes/--dry-run bypass prompts, or in JSON mode.
	if interactive && !yes && !dryRun && to == "" {
		newBase, aborted, err := promptRelocateBasePath(params.TargetBasePath)
		if err != nil {
			return err
		}
		if aborted {
			return renderRelocateAborted(cmd)
		}
		params.TargetBasePath = newBase
	}

	plan, err := worktree.PlanRelocate(params)
	if err != nil {
		return err
	}

	if !rules.PlanHasWork(plan) {
		return renderEmptyRelocate(cmd, interactive)
	}

	if dryRun {
		return renderRelocateDryRun(renderDryRunParams{
			Cmd:         cmd,
			Params:      params,
			Plan:        plan,
			Interactive: interactive,
		})
	}

	if interactive && !yes {
		// The plan is recapped by the wizard's own "Apply" step, so nothing is
		// printed before it — on abort the wizard clears and only "Aborted." remains,
		// instead of a plan preview lingering above the prompt.
		res, werr := runRelocateWizard(cfg, plan, params.BaseBranch)
		if werr != nil {
			return werr
		}
		if !res.Confirmed {
			return renderRelocateAborted(cmd)
		}
		params.Parents = res.Parents
	}

	var result domain.RelocateResult
	err = components.RunLoading(components.LoadingParams{
		Message: "Relocating worktrees…",
		Animate: interactive,
		Work:    func() error { var e error; result, e = worktree.Relocate(params); return e },
	})
	if err != nil {
		return err
	}

	if interactive {
		output.Frame(cmd.OutOrStdout(), func() {
			output.FormatRelocateResult(cmd.OutOrStdout(), result)
		})
	} else if jsonErr := output.WriteRelocateResultJSON(cmd.OutOrStdout(), result); jsonErr != nil {
		return jsonErr
	}

	if rules.RelocateHasFailure(result) {
		return domain.ErrAborted
	}
	return nil
}

// runRelocateWizard resolves the candidate parent branches and drives the
// interactive parent pickers + final apply confirmation.
func runRelocateWizard(cfg shared.ConfigResult, plan domain.RelocatePlan, baseBranch string) (relocatetui.RunResult, error) {
	return relocatetui.RunWizard(relocatetui.RunParams{
		ProjectDir: cfg.ProjectDir,
		Plan:       plan,
		Adoptions:  rules.PlanAdoptions(plan),
		Branches:   branchCandidates(cfg.ProjectDir),
		BaseBranch: baseBranch,
	})
}

type renderDryRunParams struct {
	Cmd         *cobra.Command
	Params      worktree.RelocateParams
	Plan        domain.RelocatePlan
	Interactive bool
}

// renderRelocateDryRun closes the dry-run flow: in text mode it prints the
// grouped plan and notes that nothing changed; in JSON mode it emits the preview
// result (the service performs no writes when DryRun is set).
func renderRelocateDryRun(p renderDryRunParams) error {
	if p.Interactive {
		output.FrameStart(p.Cmd.ErrOrStderr())
		output.FormatRelocatePlan(p.Cmd.ErrOrStderr(), p.Plan)
		output.Blank(p.Cmd.ErrOrStderr())
		output.Message(p.Cmd.ErrOrStderr(), "Dry run — no changes made.")
		output.FrameEnd(p.Cmd.ErrOrStderr())
		return nil
	}
	result, err := worktree.Relocate(p.Params)
	if err != nil {
		return err
	}
	return output.WriteRelocateResultJSON(p.Cmd.OutOrStdout(), result)
}

// promptRelocateBasePath runs the opt-in base_path edit and returns the resolved
// base_path and whether the user aborted (Esc). The rules validator is forwarded so
// the TUI stays free of decision logic.
func promptRelocateBasePath(current string) (basePath string, aborted bool, err error) {
	res, err := relocatetui.PromptBasePath(relocatetui.BasePathPromptParams{
		Current:  current,
		Validate: func(s string) error { return rules.ValidateRelocateTarget(s) },
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	return res.BasePath, false, nil
}

func renderRelocateAborted(cmd *cobra.Command) error {
	output.Frame(cmd.OutOrStdout(), func() {
		output.Message(cmd.OutOrStdout(), "Aborted.")
	})
	return nil
}

func resolveTargetBasePath(to string, cfg shared.ConfigResult) string {
	if to != "" {
		return to
	}
	return cfg.Config.Project.Worktrees.BasePath
}

func renderEmptyRelocate(cmd *cobra.Command, interactive bool) error {
	if !interactive {
		return output.WriteRelocateResultJSON(cmd.OutOrStdout(), domain.RelocateResult{})
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.Message(cmd.OutOrStdout(), "All worktrees are already aligned with base_path.")
	})
	return nil
}
