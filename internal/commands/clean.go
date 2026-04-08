package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	cleanui "github.com/LucasPcq/wtm/internal/tui/clean"
)

// newWtCleanCmd creates the wtm wt clean subcommand.
func newWtCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean [branch]",
		Short: "Remove a worktree and its local branch",
		Long:  "Remove a git worktree and delete the local branch. The remote branch is never touched.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Bypass all safety checks")

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	branch, err := resolveBranchArg(args, result.ProjectDir)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	cleanParams := worktree.CleanParams{
		ProjectDir: result.ProjectDir,
		Branch:     branch,
		Force:      force,
		Config:     result.Config,
	}

	if force {
		return doClean(cmd, cleanParams)
	}

	check, err := worktree.Check(cleanParams)
	if errors.Is(err, domain.ErrCannotCleanParent) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		return nil
	}
	if err != nil {
		return err
	}

	confirmResult, err := cleanui.RunConfirm(check)
	if errors.Is(err, domain.ErrUserAborted) {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}
	if err != nil {
		return err
	}

	cleanParams.Force = confirmResult.Force
	return doClean(cmd, cleanParams)
}

func resolveBranchArg(args []string, projectDir string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return cleanui.RunWorktreePicker(projectDir)
}

func doClean(cmd *cobra.Command, params worktree.CleanParams) error {
	err := worktree.Clean(params)
	if errors.Is(err, domain.ErrCannotCleanParent) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		return nil
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Cleaned worktree and branch %s\n", params.Branch)
	return nil
}
