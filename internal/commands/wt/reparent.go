package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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
		Use:   domain.CmdReparent + " [branch...]",
		Short: "Change the parent one or more worktrees are rebased onto",
		Long: "Change the recorded parent (source branch) of one or more worktrees. Only the\n" +
			"metadata is updated — the rebase happens on the next `wtm sync`. Pass the worktrees\n" +
			"and --to <parent>, or run with no arguments to multi-select interactively. The new\n" +
			"parent must exist as a local or origin remote-tracking branch (origin/x), and the\n" +
			"resulting parent chain must stay acyclic.",
		Args: cobra.ArbitraryArgs,
		RunE: runReparent,
	}

	cmd.Flags().String(domain.FlagTo, "", "New parent branch to rebase onto")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (needs at least one worktree and --to)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runReparent(cmd *cobra.Command, args []string) error {
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (the confirmation cannot run in JSON mode)", domain.FlagYes)
	}

	humanOutput := rules.IsHumanFormat(format)
	// The wizard (and its confirmation recap) needs a TTY and is skipped by --yes; a
	// human-format run without a terminal also falls back to the non-interactive path.
	// This gates the wizard only — the output format still follows --output.
	interactive := humanOutput && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	branches, newParent, err := resolveReparentTargets(resolveReparentTargetsParams{
		Branches:    args,
		NewParent:   to,
		ProjectDir:  cfg.ProjectDir,
		StateDir:    cfg.StateDir,
		Interactive: interactive,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "Aborted.")
		})
		return nil
	}
	if err != nil {
		return err
	}

	results, err := worktree.ReparentBatch(domain.ReparentBatchParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		Branches:   branches,
		NewParent:  newParent,
		BaseBranch: resolveBase("", cfg),
	})
	if err != nil {
		return err
	}

	if !humanOutput {
		return output.WriteReparentJSON(cmd.OutOrStdout(), results)
	}

	output.Frame(cmd.OutOrStdout(), func() {
		for _, result := range results {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Reparented %s: %s → %s", result.Branch, result.OldParent, result.NewParent))
		}
		output.Message(cmd.OutOrStdout(), reparentSyncHint(results))
	})
	return nil
}

// reparentSyncHint tells the user how to apply the recorded change. A single
// worktree names it in the suggested command; several point at a bare `wtm sync`.
func reparentSyncHint(results []domain.ReparentResult) string {
	if len(results) == 1 {
		return fmt.Sprintf("Run `wtm sync %s` to rebase onto the new parent.", results[0].Branch)
	}
	return "Run `wtm sync` to rebase the reparented worktrees onto their new parent."
}

type resolveReparentTargetsParams struct {
	Branches    []string
	NewParent   string
	ProjectDir  string
	StateDir    string
	Interactive bool
}

// resolveReparentTargets fills in the worktrees and new parent. A non-interactive
// run requires both (there is no picker to fall back to). An interactive run always
// opens the wizard — picking whatever is missing and confirming on a final recap,
// even when both were given as flags (mirrors create --from).
func resolveReparentTargets(params resolveReparentTargetsParams) (branches []string, newParent string, err error) {
	if !params.Interactive {
		if len(params.Branches) == 0 {
			return nil, "", fmt.Errorf("specify at least one worktree (no interactive picker under --%s, without a terminal, or in --%s %s mode)", domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		}
		if params.NewParent == "" {
			return nil, "", fmt.Errorf("specify the new parent with --%s (no interactive picker under --%s, without a terminal, or in --%s %s mode)", domain.FlagTo, domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		}
		return params.Branches, params.NewParent, nil
	}

	res, err := reparenttui.RunWizard(reparenttui.RunWizardParams{
		ProjectDir:     params.ProjectDir,
		Branches:       params.Branches,
		NewParent:      params.NewParent,
		CurrentParents: currentParents(params.ProjectDir, params.StateDir),
	})
	if err != nil {
		return nil, "", err
	}
	return res.Branches, res.NewParent, nil
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
