package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newSyncCmd creates the wtm sync subcommand.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdSync,
		Short: "Rebase every worktree onto its parent, in cascade",
		Long: "Fetch and fast-forward the base branch, then rebase each managed worktree onto its\n" +
			"parent in topological order (parents before children). The cascade is local; on a\n" +
			"conflict the branch is left clean (rebase aborted) and its descendants are skipped.\n" +
			"After a successful cascade, optionally force-push (with lease) the rebased branches.",
		Args: cobra.NoArgs,
		RunE: runSync,
	}

	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the cascade without rebasing or pushing")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the pre-sync confirmation")
	cmd.Flags().Bool(domain.FlagPush, false, "Force-push (with lease) rebased branches without prompting")
	cmd.Flags().Bool(domain.FlagNoPush, false, "Rebase locally only; never push")
	cmd.Flags().String(domain.FlagBase, "", "Base branch to sync from (defaults to config or detected base)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runSync(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	push, _ := cmd.Flags().GetBool(domain.FlagPush)
	noPush, _ := cmd.Flags().GetBool(domain.FlagNoPush)
	baseOverride, _ := cmd.Flags().GetString(domain.FlagBase)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if push && noPush {
		return fmt.Errorf("--%s and --%s are mutually exclusive", domain.FlagPush, domain.FlagNoPush)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	syncParams := worktree.SyncParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Config:     cfg.Config,
		BaseBranch: resolveBase(baseOverride, cfg),
		DryRun:     dryRun,
	}

	plan, err := worktree.PlanSync(syncParams)
	if err != nil {
		return err
	}

	interactive := format != domain.OutputJSON

	if len(plan.Steps) == 0 {
		return renderEmptyPlan(cmd, syncParams.BaseBranch, interactive)
	}

	if interactive {
		output.FormatSyncPlan(cmd.ErrOrStderr(), plan)
		if !dryRun && !yes && !confirmSync(len(plan.Steps)) {
			output.Message(cmd.ErrOrStderr(), "Aborted.")
			return nil
		}
	}

	var stop func()
	if interactive {
		stop = shared.StartSpinner(cmd.ErrOrStderr(), "Rebasing worktrees…")
	}
	syncResult, err := worktree.Sync(syncParams)
	if stop != nil {
		stop()
	}
	if err != nil {
		return err
	}

	// Recap first (text mode): the user sees what happened BEFORE being asked to push.
	if interactive {
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
		output.Blank(cmd.OutOrStdout())
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

func renderEmptyPlan(cmd *cobra.Command, base string, interactive bool) error {
	if !interactive {
		return output.WriteSyncResultJSON(cmd.OutOrStdout(), domain.SyncResult{BaseBranch: base})
	}
	output.Blank(cmd.OutOrStdout())
	output.Message(cmd.OutOrStdout(), "No worktrees to sync.")
	output.Blank(cmd.OutOrStdout())
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
