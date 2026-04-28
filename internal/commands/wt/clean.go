package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	cleanui "github.com/LucasPcq/wtm/internal/tui/clean"
)

// newCleanCmd creates the wtm wt clean subcommand.
func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdClean + " [branch]",
		Short: "Remove a worktree and its local branch",
		Long:  "Remove a git worktree and delete the local branch. The remote branch is never touched.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Bypass all safety checks")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !force {
		return fmt.Errorf("--output json requires --force (confirmations cannot run in JSON mode)")
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
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

	cleanParams := domain.CleanParams{
		ProjectDir: result.ProjectDir,
		Branch:     branch,
		Force:      force,
		Config:     result.Config,
	}

	if force {
		return doClean(cmd, cleanParams, format)
	}

	check, err := worktree.Check(cleanParams)
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		output.Blank(cmd.ErrOrStderr())
		return nil
	}
	if err != nil {
		return err
	}

	confirmResult, err := cleanui.RunConfirm(check)
	if errors.Is(err, domain.ErrUserAborted) {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "Aborted.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}
	if err != nil {
		return err
	}

	cleanParams.Force = confirmResult.Force
	return doClean(cmd, cleanParams, format)
}

func resolveBranchArg(args []string, projectDir string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return cleanui.RunWorktreePicker(projectDir)
}

func doClean(cmd *cobra.Command, params domain.CleanParams, format string) error {
	wtPath := ""
	if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		wtPath = wt.Path
	}

	stopWorktreeServices(cmd, params.ProjectDir, params.Branch)

	err := worktree.Clean(params)
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		output.Blank(cmd.ErrOrStderr())
		return nil
	}
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
			Branch: params.Branch,
			Path:   wtPath,
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Cleaned worktree and branch %s", params.Branch))
	output.Blank(cmd.OutOrStdout())
	return nil
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
