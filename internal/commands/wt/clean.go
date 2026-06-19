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

	cleanParams := domain.CleanParams{
		ProjectDir: result.ProjectDir,
		Branch:     branch,
		Force:      force,
		Config:     result.Config,
	}

	if force {
		return doClean(cmd, cleanParams, format)
	}

	stopCheck := shared.StartSpinner(cmd.ErrOrStderr(), "Checking worktree…")
	check, err := worktree.Check(cleanParams)
	stopCheck()
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", branch))
		output.Blank(cmd.OutOrStdout())
		return nil
	}
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

	// Decide before removal (paths must still exist to resolve symlinks).
	cwd, _ := os.Getwd()
	insideRemoved := wtPath != "" && cwd != "" && rules.IsPathWithin(resolveSymlinks(wtPath), resolveSymlinks(cwd))

	stopWorktreeServices(cmd, params.ProjectDir, params.Branch)

	var stop func()
	if format != domain.OutputJSON {
		stop = shared.StartSpinner(cmd.ErrOrStderr(), "Cleaning worktree…")
	}
	err := worktree.Clean(params)
	if stop != nil {
		stop()
	}
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		// Idempotent: cleaning an absent worktree is a no-op success so agents
		// can safely retry.
		if format == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
				Branch:        params.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", params.Branch))
		output.Blank(cmd.OutOrStdout())
		return nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		output.Blank(cmd.ErrOrStderr())
		return nil
	}
	if err != nil {
		return err
	}

	if insideRemoved {
		redirectToBase(params.ProjectDir)
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
