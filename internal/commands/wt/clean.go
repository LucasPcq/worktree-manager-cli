package wt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	cleanui "github.com/LucasPcq/wtm/internal/tui/clean"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newCleanCmd creates the wtm clean subcommand.
func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdClean + " [branch]",
		Short: "Remove a worktree and its local branch",
		Long:  "Remove a git worktree and delete the local branch. The remote branch is never touched.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Bypass all safety checks")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, "Reparent orphaned child worktrees onto the grandparent (no prompt)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	reparentFlag, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !force {
		return fmt.Errorf("--output json requires --force (confirmations cannot run in JSON mode)")
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	branch, err := resolveBranchArg(args, result.ProjectDir)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	interactive := rules.IsHumanFormat(format)
	cleanParams := domain.CleanParams{
		ProjectDir: result.ProjectDir,
		StateDir:   result.StateDir,
		Branch:     branch,
		Force:      force,
		BaseBranch: resolveBase("", result),
		Config:     result.Config,
	}

	reparentPlan := worktree.PlanCleanReparent(cleanParams)

	if !force {
		var check domain.CleanCheckResult
		err = components.RunLoading(components.LoadingParams{
			Message: "Checking worktree…",
			Animate: interactive,
			Work:    func() error { var e error; check, e = worktree.Check(cleanParams); return e },
		})
		if errors.Is(err, domain.ErrWorktreeNotFound) {
			output.Frame(cmd.OutOrStdout(), func() {
				output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", branch))
			})
			return nil
		}
		if errors.Is(err, domain.ErrCannotCleanParent) {
			output.Frame(cmd.ErrOrStderr(), func() {
				output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
			})
			return nil
		}
		if err != nil {
			return err
		}

		// Open the confirm-section frame here so framing stays in the command
		// layer; RunConfirm renders only its raw body. The result section's own
		// output.Frame (in doClean) supplies the trailing blank.
		output.FrameStart(cmd.ErrOrStderr())
		confirmResult, err := cleanui.RunConfirm(check)
		if errors.Is(err, domain.ErrUserAborted) {
			output.Frame(cmd.OutOrStdout(), func() {
				output.Message(cmd.OutOrStdout(), "Aborted.")
			})
			return nil
		}
		if err != nil {
			return err
		}
		cleanParams.Force = confirmResult.Force
	}

	applyReparent, aborted := decideReparent(cmd, reparentPlan, reparentFlag, interactive)
	if aborted {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "Aborted.")
		})
		return nil
	}
	return doClean(cmd, cleanParams, format, reparentPlan, applyReparent)
}

// decideReparent resolves whether the cleaned worktree's orphaned children should
// be reparented onto the grandparent. With no children it is a no-op. The flag is
// the explicit permission in non-interactive mode; otherwise the user is asked
// with a recap of exactly what would change. A second return value reports an Esc
// on the proposal, which aborts the whole clean (nothing is deleted).
func decideReparent(cmd *cobra.Command, plan domain.CleanReparentPlan, flag bool, interactive bool) (apply bool, abort bool) {
	if len(plan.Children) == 0 {
		return false, false
	}
	if flag {
		return true, false
	}
	if !interactive {
		return false, false
	}
	return confirmReparent(cmd, plan)
}

// confirmReparent shows the proposed reparenting (a bold section, separated from
// the "Will delete" recap by a blank line) and asks. Yes → reparent; No → delete
// but leave the children orphaned; Esc → abort the whole clean.
func confirmReparent(cmd *cobra.Command, plan domain.CleanReparentPlan) (apply bool, abort bool) {
	output.FormatReparentProposal(cmd.ErrOrStderr(), plan)

	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("Reparent %d child worktree(s) onto %s?", len(plan.Children), plan.Grandparent),
		DefaultYes: true,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	if err != nil {
		// Esc (or a program failure) aborts the whole operation rather than
		// silently deleting the parent and orphaning its children.
		return false, true
	}
	return confirmed, false
}

func resolveBranchArg(args []string, projectDir string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return cleanui.RunWorktreePicker(projectDir)
}

func doClean(cmd *cobra.Command, params domain.CleanParams, format string, reparentPlan domain.CleanReparentPlan, applyReparent bool) error {
	wtPath := ""
	if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		wtPath = wt.Path
	}

	// Decide before removal (paths must still exist to resolve symlinks).
	cwd, _ := os.Getwd()
	insideRemoved := wtPath != "" && cwd != "" && rules.IsPathWithin(resolveSymlinks(wtPath), resolveSymlinks(cwd))

	stopWorktreeServices(cmd, params.ProjectDir, params.Branch)

	err := components.RunLoading(components.LoadingParams{
		Message: "Cleaning worktree…",
		Animate: rules.IsHumanFormat(format),
		Work:    func() error { return worktree.Clean(params) },
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		// Idempotent: cleaning an absent worktree is a no-op success so agents
		// can safely retry.
		if format == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
				Branch:        params.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", params.Branch))
		})
		return nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Frame(cmd.ErrOrStderr(), func() {
			output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		})
		return nil
	}
	if err != nil {
		return err
	}

	if insideRemoved {
		redirectToBase(params.ProjectDir)
	}

	reparented, reparentErr := applyChildReparent(params.StateDir, reparentPlan, applyReparent)
	if reparentErr != nil {
		return reparentErr
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
			Branch:           params.Branch,
			Path:             wtPath,
			Reparented:       reparented,
			OrphanedChildren: orphanedChildren(reparentPlan, applyReparent),
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Cleaned worktree and branch %s", params.Branch))
		for _, child := range reparented {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Reparented %s onto %s", child.Branch, child.NewParent))
		}
		for _, child := range orphanedChildren(reparentPlan, applyReparent) {
			output.Warning(cmd.OutOrStdout(), fmt.Sprintf("%s still points at the removed parent %s — reparent it with `wtm reparent`", child.Branch, child.OldParent))
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

// orphanedChildren returns the children left dangling because reparenting was not
// authorized.
func orphanedChildren(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}

// redirectToBase asks the shell wrapper to cd into the base repo, avoiding a
// stale "ghost" directory after removing the worktree we were sitting in. It
// relies on the WTM_GO_FILE bridge (see wtm shell-init); a no-op without it.
func redirectToBase(baseDir string) {
	goFile := os.Getenv(domain.EnvGoFile)
	if goFile == "" {
		return
	}
	_ = os.WriteFile(goFile, []byte(baseDir), 0o644)
}

// resolveSymlinks returns the canonical path, falling back to the input when it
// cannot be resolved (e.g. the path no longer exists).
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func stopWorktreeServices(cmd *cobra.Command, projectDir string, branch string) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return
	}

	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: projectDir,
		Branch:     branch,
	})
	if err != nil {
		return
	}

	client := process.NewClient(socketPath)
	if process.StopWorktreeJobs(client, wt.Path) {
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf("Stopped services on %s", branch))
	}
}
