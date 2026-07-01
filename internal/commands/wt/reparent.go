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
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	reparenttui "github.com/LucasPcq/wtm/internal/tui/reparent"
)

// newReparentCmd creates the wtm reparent subcommand.
func newReparentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdReparent + " [branch]",
		Short: "Change the parent a worktree is rebased onto",
		Long: "Change the recorded parent (source branch) of a worktree. Only the metadata is\n" +
			"updated — the rebase happens on the next `wtm sync`. Pass the worktree and --to <parent>,\n" +
			"or run with no arguments to pick interactively. The new parent must exist as a local or\n" +
			"origin remote-tracking branch (origin/x), and the resulting parent chain must stay acyclic.",
		Args: cobra.MaximumNArgs(1),
		RunE: runReparent,
	}

	cmd.Flags().String(domain.FlagTo, "", "New parent branch to rebase onto")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runReparent(cmd *cobra.Command, args []string) error {
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := rules.IsHumanFormat(format)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	branch := firstArg(args)

	branch, newParent, err := resolveReparentTargets(resolveReparentTargetsParams{
		Branch:      branch,
		NewParent:   to,
		ProjectDir:  cfg.ProjectDir,
		StateDir:    cfg.StateDir,
		Interactive: interactive,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	result, err := worktree.Reparent(domain.ReparentParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Branch:     branch,
		NewParent:  newParent,
		BaseBranch: resolveBase("", cfg),
	})
	if err != nil {
		return err
	}

	if !interactive {
		return output.WriteReparentJSON(cmd.OutOrStdout(), result)
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Reparented %s: %s → %s", result.Branch, result.OldParent, result.NewParent))
		output.Message(cmd.OutOrStdout(), fmt.Sprintf("Run `wtm sync %s` to rebase onto the new parent.", result.Branch))
	})
	return nil
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

type resolveReparentTargetsParams struct {
	Branch      string
	NewParent   string
	ProjectDir  string
	StateDir    string
	Interactive bool
}

// resolveReparentTargets fills in the worktree and new parent. When both are
// already supplied it returns them untouched. Otherwise an interactive run opens
// the multi-step wizard for whichever is missing; a non-interactive run is a usage
// error (there is no picker to fall back to).
func resolveReparentTargets(params resolveReparentTargetsParams) (branch, newParent string, err error) {
	if params.Branch != "" && params.NewParent != "" {
		return params.Branch, params.NewParent, nil
	}

	if !params.Interactive {
		if params.Branch == "" {
			return "", "", fmt.Errorf("specify a worktree (no interactive picker in --%s %s mode)", domain.FlagOutput, domain.OutputJSON)
		}
		return "", "", fmt.Errorf("specify the new parent with --%s (no interactive picker in --%s %s mode)", domain.FlagTo, domain.FlagOutput, domain.OutputJSON)
	}

	res, err := reparenttui.RunWizard(reparenttui.RunWizardParams{
		ProjectDir:     params.ProjectDir,
		Branch:         params.Branch,
		NewParent:      params.NewParent,
		CurrentParents: currentParents(params.ProjectDir, params.StateDir),
	})
	if err != nil {
		return "", "", err
	}
	return res.Branch, res.NewParent, nil
}

// currentParents maps each non-main worktree to its recorded parent, so the
// wizard can show what a branch is currently rebased onto. A worktree with no
// metadata maps to the empty string.
func currentParents(projectDir, stateDir string) map[string]string {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil
	}
	parents := make(map[string]string, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		parents[wt.Branch] = worktree.ParentBranch(worktree.ParentBranchParams{
			StateDir: stateDir,
			Branch:   wt.Branch,
		})
	}
	return parents
}
