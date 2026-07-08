// Package prune is the Factory-pattern implementation of `wtm prune`: a thin
// adapter that resolves flags into PruneOptions, completes and validates them, then
// delegates to the pure worktree service. Its bypass flags (--yes / --force /
// --dry-run) behave exactly like clean's (see MIGRATION_GUIDE_COMMAND_FACTORY.md).
package prune

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/cmd/prune/wizard"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

// PruneOptions is the single contract the run layer consumes. Every flag lands in a
// typed field here; the run layer never reads raw cobra flags.
type PruneOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Progress progress.Runner
	Wizard   wizard.Wizard
	Config   func(dir string) (shared.ConfigResult, error)

	// Inputs (flags).
	Merged           bool
	Closed           bool
	Gone             bool
	NoFetch          bool
	Force            bool
	ReparentChildren bool
	Yes              bool
	DryRun           bool
	Output           string

	// Resolved state (filled by completeOptions).
	Interactive bool
	cfg         shared.ConfigResult
	baseBranch  string
}

// NewCmdPrune wires the command. w is prune's interactive-flow port (injected at the
// composition root so the command never imports tea/pruneui). runF is the test
// injection point: nil in prod (the real pruneRun runs), a capturing closure in tests.
func NewCmdPrune(f *cmdutil.Factory, w wizard.Wizard, runF func(*PruneOptions) error) *cobra.Command {
	opts := &PruneOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Progress: f.Progress,
		Wizard:   w,
		Config:   f.Config,
	}

	cmd := &cobra.Command{
		Use:   domain.CmdPrune,
		Short: "Remove finished worktrees (merged, closed PR, gone, or old) in one pass",
		Long: "Batch-remove worktrees whose work is done, reparenting any surviving children onto\n" +
			"their grandparent (like `clean --reparent-children`). Whether work is \"done\" is read\n" +
			"from GitHub via the `gh` CLI — never guessed from local commits — so squash- and\n" +
			"rebase-merges are detected correctly. By default prune considers every finished\n" +
			"worktree: merged PR, closed PR, or upstream branch gone. The reason flags restrict to\n" +
			"specific categories — --merged (PR merged), --closed (PR closed unmerged), --gone\n" +
			"(remote branch deleted).\n" +
			"\n" +
			"--merged and --closed require the GitHub CLI (`gh`) to be installed and authenticated;\n" +
			"without it they match nothing and prune prints a notice — only --gone still applies.\n" +
			"gone-detection runs `git fetch --prune` first so deleted remote branches are seen\n" +
			"(pass --no-fetch to skip).\n" +
			"\n" +
			"On a TTY, matches are shown for review (unsafe ones unchecked), then a prune\n" +
			"confirmation, then — like clean — a dedicated confirmation to reparent surviving\n" +
			"children onto their grandparent (or leave them orphaned). The main worktree and base\n" +
			"branch are always protected; the current worktree is removed and the shell\n" +
			"redirected to the base repo. Like clean, worktrees that are dirty, have unpushed\n" +
			"commits, or have an open PR are unsafe and need --force. Use --yes to skip the\n" +
			"prompts (required with --output json); non-interactively, children are left orphaned\n" +
			"unless --reparent-children is passed. --dry-run previews without changing anything.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := completeOptions(cmd, opts); err != nil {
				return err
			}
			if err := validateOptions(opts); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return pruneRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Merged, domain.FlagMerged, false,
		"Restrict to worktrees whose PR was merged on GitHub (needs gh)")
	cmd.Flags().BoolVar(&opts.Closed, domain.FlagClosed, false,
		"Restrict to worktrees whose PR was closed without merging (needs gh)")
	cmd.Flags().BoolVar(&opts.Gone, domain.FlagGone, false,
		"Restrict to worktrees whose upstream branch was deleted on the remote")
	cmd.Flags().BoolVar(&opts.NoFetch, domain.FlagNoFetch, false,
		"Skip the git fetch --prune that gone-detection performs; use already-fetched state")
	cmd.Flags().BoolVar(&opts.Force, domain.FlagForce, false,
		"Lift safety refusals (dirty/unpushed/open-PR): also remove unsafe worktrees; still asks to confirm unless --yes")
	cmd.Flags().BoolVar(&opts.ReparentChildren, domain.FlagReparentChildren, false,
		"Reparent orphaned child worktrees onto the grandparent (no prompt)")
	cmd.Flags().BoolVarP(&opts.Yes, domain.FlagYes, "y", false,
		"Skip all prompts; keep every match without the selection picker (use --force for unsafe worktrees)")
	cmd.Flags().BoolVar(&opts.DryRun, domain.FlagDryRun, false,
		"Preview what would be pruned without removing anything")
	shared.AddOutputFlag(cmd)

	return cmd
}

// completeOptions resolves interactivity, the default-broad reasons, and config. It
// does NOT prompt field-by-field: interactive collection is the picker's job (pruneRun).
func completeOptions(cmd *cobra.Command, opts *PruneOptions) error {
	opts.Output, _ = cmd.Flags().GetString(domain.FlagOutput)

	// The --yes fold: --yes turns off the whole interactive mode (collection AND
	// confirmation), mirroring clean. So `prune --yes` never opens the picker.
	opts.Interactive = opts.IO.CanPrompt() && !opts.Yes

	// Default is broad: with no reason flag, consider every finished worktree
	// (merged, closed-PR, or gone). Reason flags narrow to specific categories.
	if !opts.Merged && !opts.Closed && !opts.Gone {
		opts.Merged, opts.Closed, opts.Gone = true, true, true
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg, err := opts.Config(dir)
	if err != nil {
		return err
	}
	opts.cfg = cfg
	opts.baseBranch = shared.ResolveBase("", cfg)
	return nil
}

// validateOptions is the safety net common to both modes. The "needs a terminal"
// gate is enforced by cmdutil.Confirm in the non-interactive run path (like clean).
func validateOptions(opts *PruneOptions) error {
	if opts.Output == domain.OutputJSON && !opts.Yes && !opts.DryRun {
		return cmdutil.FlagErrorf("--output json requires --yes or --dry-run (the selection prompt cannot run in JSON mode)")
	}
	return nil
}
