package wt

import (
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

	plan, err := worktree.PlanRelocate(params)
	if err != nil {
		return err
	}

	interactive := rules.IsHumanFormat(format)

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
		// Frame top lives on stderr alongside the plan. With --yes there is no
		// plan, so the frame opens on stdout via the result's leading blank below
		// (mirrors sync), avoiding a blank stacked against an empty stderr section.
		output.FrameStart(cmd.ErrOrStderr())
		output.FormatRelocatePlan(cmd.ErrOrStderr(), plan)
		// No separator blank here: the wizard owns its leading blank.
		res, werr := runRelocateWizard(cfg, plan, params.BaseBranch)
		if werr != nil {
			return werr
		}
		if !res.Confirmed {
			output.Message(cmd.ErrOrStderr(), "Aborted.")
			output.FrameEnd(cmd.ErrOrStderr())
			return nil
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
		// The blank separates the plan/spinner section (stderr) from the result
		// (stdout); it replaces the spinner's former self-padding.
		output.Blank(cmd.OutOrStdout())
		output.FormatRelocateResult(cmd.OutOrStdout(), result)
		output.FrameEnd(cmd.OutOrStdout())
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
