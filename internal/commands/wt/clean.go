package wt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/shell"
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

	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, "Reparent orphaned child worktrees onto the grandparent (no prompt)")
	shared.AddOutputFlag(cmd)

	return cmd
}

// runClean wires the flags into the clean flow: it decides who may be asked and
// where the output goes, and owns no part of the déroulé itself (internal/flow).
func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	reparentFlag, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return domain.ErrCleanJSONNeedsYes
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	branchName := ""
	if len(args) > 0 {
		branchName = args[0]
	}

	// The prompt-capability gate: a human format on a real terminal AND not --yes.
	// --force is the safety axis, not a confirmation bypass — it still runs the
	// wizard, with the refusals lifted.
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes

	presenter := newPresenter(cmd, format)
	_, err = flow.Clean(flow.CleanParams{
		Context: flowContext(config),
		Request: flow.CleanRequest{
			Branch:           branchName,
			Force:            force,
			ReparentChildren: reparentFlag,
			BaseBranch:       resolveBase("", config),
		},
		// The picker may be reached through the shell wrapper, which consumes stdout,
		// so the wizard renders on stderr.
		Prompter:  flowPrompter(flowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: cleanPresenter{cliPresenter: presenter},
	})
	return err
}

// The helpers below are shared with `wtm prune`, which still removes worktrees on
// its own. They go away with its migration to internal/flow.

// redirectToBase asks the shell wrapper to cd into the base repo, avoiding a
// stale "ghost" directory after removing the worktree we were sitting in.
func redirectToBase(baseDir string) {
	shell.RequestCd(baseDir)
}

// resolveSymlinks returns the canonical path, falling back to the input when it
// cannot be resolved (e.g. the path no longer exists).
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// stopWorktreeServices asks the daemon to stop the jobs running in a worktree
// about to disappear.
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
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf(domain.CleanStoppedServicesFmt, branch))
	}
}
