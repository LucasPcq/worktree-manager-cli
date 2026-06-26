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
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/syncpicker"
)

// newSyncCmd creates the wtm sync subcommand.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdSync + " [branch...]",
		Short: "Rebase selected worktrees onto their parent, in cascade",
		Long: "Rebase one or more managed worktrees onto their parent. Pass branch names to target\n" +
			"specific worktrees, --all to sync every worktree, or no arguments to pick interactively.\n" +
			"The base branch is fetched and fast-forwarded first, then each selected worktree is\n" +
			"rebased onto its parent in topological order (parents before children). The cascade is\n" +
			"local; on a conflict the branch is left clean (rebase aborted) and its selected\n" +
			"descendants are skipped. After a successful cascade, optionally force-push (with lease)\n" +
			"the rebased branches.",
		Args: cobra.ArbitraryArgs,
		RunE: runSync,
	}

	cmd.Flags().Bool(domain.FlagAll, false, "Sync every managed worktree")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the cascade without rebasing or pushing")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the pre-sync confirmation")
	cmd.Flags().Bool(domain.FlagPush, false, "Force-push (with lease) rebased branches without prompting")
	cmd.Flags().Bool(domain.FlagNoPush, false, "Rebase locally only; never push")
	cmd.Flags().String(domain.FlagBase, "", "Base branch to sync from (defaults to config or detected base)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	push, _ := cmd.Flags().GetBool(domain.FlagPush)
	noPush, _ := cmd.Flags().GetBool(domain.FlagNoPush)
	baseOverride, _ := cmd.Flags().GetString(domain.FlagBase)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if push && noPush {
		return fmt.Errorf("--%s and --%s are mutually exclusive", domain.FlagPush, domain.FlagNoPush)
	}
	if all && len(args) > 0 {
		return fmt.Errorf("--%s cannot be combined with branch arguments", domain.FlagAll)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	baseBranch := resolveBase(baseOverride, cfg)
	interactive := rules.IsHumanFormat(format)

	selected, err := resolveSyncSelection(resolveSyncSelectionParams{
		Args:        args,
		All:         all,
		Interactive: interactive,
		Cfg:         cfg,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	syncParams := worktree.SyncParams{
		ProjectDir:       cfg.ProjectDir,
		StateDir:         cfg.StateDir,
		Config:           cfg.Config,
		BaseBranch:       baseBranch,
		DryRun:           dryRun,
		SelectedBranches: selected,
	}

	plan, err := worktree.PlanSync(syncParams)
	if err != nil {
		return err
	}

	if len(plan.Steps) == 0 && !syncIncludesBase(selected, baseBranch) {
		return renderEmptyPlan(cmd, syncParams.BaseBranch, interactive)
	}

	if interactive && len(plan.Steps) > 0 {
		output.FrameStart(cmd.ErrOrStderr())
		output.FormatSyncPlan(cmd.ErrOrStderr(), plan)
		if !dryRun && !yes && !confirmSync(len(plan.Steps)) {
			output.Message(cmd.ErrOrStderr(), "Aborted.")
			output.FrameEnd(cmd.ErrOrStderr())
			return nil
		}
	}

	var syncResult domain.SyncResult
	err = components.RunLoading(components.LoadingParams{
		Message: "Rebasing worktrees…",
		Animate: interactive,
		Work:    func() error { var e error; syncResult, e = worktree.Sync(syncParams); return e },
	})
	if err != nil {
		return err
	}

	// Recap first (text mode): the user sees what happened BEFORE being asked to push.
	// The blank separates the plan/spinner section (stderr) from the recap (stdout);
	// it replaces the spinner's former self-padding.
	if interactive {
		output.Blank(cmd.OutOrStdout())
		output.FormatSyncResult(cmd.OutOrStdout(), syncResult)
	}

	if !dryRun && decidePush(decidePushParams{
		Push:        push,
		NoPush:      noPush,
		Interactive: interactive,
		Steps:       syncResult.Steps,
	}) {
		syncResult = worktree.PushSynced(worktree.PushSyncedParams{
			ProjectDir: cfg.ProjectDir,
			Result:     syncResult,
		})
	}

	if interactive {
		output.FormatSyncPushSummary(cmd.OutOrStdout(), syncResult.Steps)
		output.FrameEnd(cmd.OutOrStdout())
	} else if err := output.WriteSyncResultJSON(cmd.OutOrStdout(), syncResult); err != nil {
		return err
	}

	if !dryRun && hasSyncFailure(syncResult.Steps) {
		return domain.ErrAborted
	}
	return nil
}

func resolveBase(override string, cfg shared.ConfigResult) string {
	if override != "" {
		return override
	}
	if cfg.Config.Project.Worktrees.BaseBranch != "" {
		return cfg.Config.Project.Worktrees.BaseBranch
	}
	return detect.BaseBranch(cfg.ProjectDir)
}

type resolveSyncSelectionParams struct {
	Args        []string
	All         bool
	Interactive bool
	Cfg         shared.ConfigResult
}

// resolveSyncSelection turns the CLI inputs into the list of branches to sync.
// A nil result means "every worktree" (the --all case). Positional args are
// resolved to concrete branches; with no args, an interactive run opens the
// multi-select picker and a non-interactive run is a usage error.
func resolveSyncSelection(params resolveSyncSelectionParams) ([]string, error) {
	if params.All {
		return nil, nil
	}

	if len(params.Args) > 0 {
		return worktree.ResolveSyncBranches(worktree.ResolveSyncBranchesParams{
			ProjectDir: params.Cfg.ProjectDir,
			Queries:    params.Args,
		})
	}

	if !params.Interactive {
		return nil, fmt.Errorf("specify one or more worktrees, or pass --%s (no interactive picker in --%s %s mode)",
			domain.FlagAll, domain.FlagOutput, domain.OutputJSON)
	}

	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: params.Cfg.ProjectDir,
		StateDir:   params.Cfg.StateDir,
		Config:     params.Cfg.Config,
	})
	if err != nil {
		return nil, err
	}

	return syncpicker.Run(syncpicker.RunParams{Statuses: statuses})
}

// syncIncludesBase reports whether the selection asks to refresh the base branch.
// The --all case (nil selection) always does; an explicit selection does when it
// names the base. It lets the command run a base-only refresh with no rebase steps.
func syncIncludesBase(selected []string, baseBranch string) bool {
	if selected == nil {
		return true
	}
	for _, branch := range selected {
		if branch == baseBranch {
			return true
		}
	}
	return false
}

func renderEmptyPlan(cmd *cobra.Command, base string, interactive bool) error {
	if !interactive {
		return output.WriteSyncResultJSON(cmd.OutOrStdout(), domain.SyncResult{BaseBranch: base})
	}
	output.Frame(cmd.OutOrStdout(), func() {
		output.Message(cmd.OutOrStdout(), "No worktrees to sync.")
	})
	return nil
}

func confirmSync(count int) bool {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("Rebase %d worktree(s) onto their parents?", count),
		DefaultYes: true,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	return err == nil && confirmed
}

type decidePushParams struct {
	Push        bool
	NoPush      bool
	Interactive bool
	Steps       []domain.SyncStepResult
}

// decidePush resolves whether to push the rebased branches. With branches to
// push and neither --push nor --no-push, an interactive run asks once; a
// non-interactive run only pushes when --push is set.
func decidePush(params decidePushParams) bool {
	ready := pushableCount(params.Steps)
	if ready == 0 || params.NoPush {
		return false
	}
	if params.Push {
		return true
	}
	if !params.Interactive {
		return false
	}
	return confirmPush(ready)
}

func confirmPush(count int) bool {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("Push %d rebased branch(es) to origin?", count),
		Warning:    "This rewrites the remote branches with --force-with-lease.",
		DefaultYes: false,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	return err == nil && confirmed
}

func pushableCount(steps []domain.SyncStepResult) int {
	count := 0
	for _, step := range steps {
		if step.PushPending && !step.Pushed {
			count++
		}
	}
	return count
}

func hasSyncFailure(steps []domain.SyncStepResult) bool {
	for _, step := range steps {
		if step.Status == domain.SyncStatusConflict || step.Status == domain.SyncStatusError {
			return true
		}
	}
	return false
}
