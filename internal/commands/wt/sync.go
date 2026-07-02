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
			"descendants are skipped. Pass --keep-conflict to leave a conflicting rebase in progress\n" +
			"in its worktree for manual resolution instead of aborting. After a successful cascade,\n" +
			"optionally force-push (with lease) the rebased branches.",
		Args: cobra.ArbitraryArgs,
		RunE: runSync,
	}

	cmd.Flags().Bool(domain.FlagAll, false, "Sync every managed worktree")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the cascade without rebasing or pushing")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the pre-sync confirmation")
	cmd.Flags().Bool(domain.FlagPush, false, "Force-push (with lease) rebased branches without prompting")
	cmd.Flags().Bool(domain.FlagNoPush, false, "Rebase locally only; never push")
	cmd.Flags().Bool(domain.FlagKeepConflict, false, "Leave a conflicting rebase in progress in its worktree instead of aborting")
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
	keepConflict, _ := cmd.Flags().GetBool(domain.FlagKeepConflict)
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

	// planPreview lets the interactive picker render the cascade preview on its
	// confirmation step without importing the service/output packages. It plans
	// against the in-progress selection; conflict mode does not affect the plan.
	planTemplate := worktree.SyncParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Config:     cfg.Config,
		BaseBranch: baseBranch,
		DryRun:     dryRun,
	}
	planPreview := func(p syncpicker.PlanPreviewParams) (string, int, error) {
		preview := planTemplate
		preview.SelectedBranches = p.Branches
		plan, err := worktree.PlanSync(preview)
		if err != nil {
			return "", 0, err
		}
		return output.SprintSyncPlan(plan), len(plan.Steps), nil
	}

	selection, err := resolveSyncSelection(resolveSyncSelectionParams{
		Args:         args,
		All:          all,
		KeepConflict: keepConflict,
		CanPrompt:    interactive && term.IsTerminal(int(os.Stdin.Fd())),
		Cfg:          cfg,
		BaseBranch:   baseBranch,
		PlanPreview:  planPreview,
		SkipConfirm:  dryRun || yes,
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
		KeepConflict:     selection.KeepConflict,
		SelectedBranches: selection.Branches,
	}

	plan, err := worktree.PlanSync(syncParams)
	if err != nil {
		return err
	}

	if len(plan.Steps) == 0 && !rules.SyncIncludesBase(rules.SyncIncludesBaseParams{Selected: selection.Branches, BaseBranch: baseBranch}) {
		return renderEmptyPlan(cmd, syncParams.BaseBranch, interactive)
	}

	if interactive && len(plan.Steps) > 0 {
		output.FrameStart(cmd.ErrOrStderr())
		// The interactive picker already previewed the plan and confirmed it on its
		// own step (see syncpicker); only the non-picker flow previews/confirms here.
		if !selection.PlanConfirmed {
			output.FormatSyncPlan(cmd.ErrOrStderr(), plan)
			if !dryRun && !yes && !confirmSync(confirmSyncParams{Count: len(plan.Steps), KeepConflict: syncParams.KeepConflict}) {
				output.Message(cmd.ErrOrStderr(), "Aborted.")
				output.FrameEnd(cmd.ErrOrStderr())
				return nil
			}
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

	if !dryRun && shouldPush(shouldPushParams{
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

	if !dryRun && rules.HasSyncFailure(syncResult.Steps) {
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
	Args         []string
	All          bool
	KeepConflict bool
	CanPrompt    bool
	Cfg          shared.ConfigResult
	// BaseBranch, PlanPreview and SkipConfirm are forwarded to the interactive
	// picker so its confirmation step can preview the cascade (see syncpicker).
	BaseBranch  string
	PlanPreview func(syncpicker.PlanPreviewParams) (string, int, error)
	SkipConfirm bool
}

// syncSelection is the resolved sync target: the branches to rebase (nil means
// "every worktree"), whether a conflicting rebase is left in progress, and
// whether the plan was already previewed and confirmed interactively (by the
// picker) so runSync must not preview/confirm it again.
type syncSelection struct {
	Branches      []string
	KeepConflict  bool
	PlanConfirmed bool
}

// resolveSyncSelection turns the CLI inputs into the branches to sync and the
// conflict mode. Positional args and --all take the conflict mode from the
// --keep-conflict flag. With no args, the two-step picker opens when the session
// can prompt (human format on a TTY) — its second step decides the conflict mode
// (pre-selected from the flag) — and is otherwise a usage error.
func resolveSyncSelection(params resolveSyncSelectionParams) (syncSelection, error) {
	if params.All {
		return syncSelection{Branches: nil, KeepConflict: params.KeepConflict}, nil
	}

	if len(params.Args) > 0 {
		branches, err := worktree.ResolveSyncBranches(worktree.ResolveSyncBranchesParams{
			ProjectDir: params.Cfg.ProjectDir,
			Queries:    params.Args,
		})
		if err != nil {
			return syncSelection{}, err
		}
		return syncSelection{Branches: branches, KeepConflict: params.KeepConflict}, nil
	}

	if !params.CanPrompt {
		return syncSelection{}, fmt.Errorf("specify one or more worktrees, or pass --%s (no interactive picker without a terminal or in --%s %s mode)",
			domain.FlagAll, domain.FlagOutput, domain.OutputJSON)
	}

	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: params.Cfg.ProjectDir,
		StateDir:   params.Cfg.StateDir,
		Config:     params.Cfg.Config,
	})
	if err != nil {
		return syncSelection{}, err
	}

	result, err := syncpicker.Run(syncpicker.RunParams{
		Statuses:            statuses,
		DefaultKeepConflict: params.KeepConflict,
		BaseBranch:          params.BaseBranch,
		PlanPreview:         params.PlanPreview,
		SkipConfirm:         params.SkipConfirm,
	})
	if err != nil {
		return syncSelection{}, err
	}
	// Declining the plan on the picker's confirmation step aborts, like Esc.
	if !result.Confirmed {
		return syncSelection{}, domain.ErrUserAborted
	}
	return syncSelection{
		Branches:      result.Branches,
		KeepConflict:  result.KeepConflict,
		PlanConfirmed: !params.SkipConfirm,
	}, nil
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

type confirmSyncParams struct {
	Count        int
	KeepConflict bool
}

func confirmSync(params confirmSyncParams) bool {
	warning := ""
	if params.KeepConflict {
		warning = domain.SyncKeepConflictWarning
	}
	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.SyncConfirmPrompt, params.Count),
		Warning:    warning,
		DefaultYes: true,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	return err == nil && confirmed
}

type shouldPushParams struct {
	Push        bool
	NoPush      bool
	Interactive bool
	Steps       []domain.SyncStepResult
}

// shouldPush resolves the pure push decision (in rules) and, when interactive
// confirmation is required, runs the TUI prompt.
func shouldPush(params shouldPushParams) bool {
	ready := rules.PushableCount(params.Steps)
	switch rules.DecidePush(rules.DecidePushParams{
		Push:          params.Push,
		NoPush:        params.NoPush,
		Interactive:   params.Interactive,
		PushableCount: ready,
	}) {
	case rules.PushForce:
		return true
	case rules.PushConfirm:
		return confirmPush(ready)
	default:
		return false
	}
}

func confirmPush(count int) bool {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.SyncPushPrompt, count),
		Warning:    domain.SyncPushWarning,
		DefaultYes: false,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	return err == nil && confirmed
}

