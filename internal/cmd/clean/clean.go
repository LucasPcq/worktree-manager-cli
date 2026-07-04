// Package clean is the Factory-pattern implementation of `wtm clean`: a thin
// adapter that resolves flags into CleanOptions, completes and validates them, then
// delegates to the pure worktree service. It is the pilot for the Command / Options
// / Run + Factory architecture (see MIGRATION_GUIDE_COMMAND_FACTORY.md).
package clean

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/cmd/clean/wizard"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

// CleanOptions is the single contract the run layer consumes. Every flag lands in a
// typed field here; the run layer never reads raw cobra flags.
type CleanOptions struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Progress progress.Runner
	Wizard   wizard.Wizard
	Config   func(dir string) (shared.ConfigResult, error)

	// Inputs (flags / positional arg).
	Branch           string
	Force            bool
	Yes              bool
	ReparentChildren bool
	DryRun           bool
	Output           string

	// Resolved state (filled by completeOptions).
	Interactive bool
	cfg         shared.ConfigResult
	baseBranch  string
}

// NewCmdClean wires the command. w is clean's interactive-flow port (injected at the
// composition root so the command never imports tea/cleanui). runF is the test
// injection point: nil in prod (the real cleanRun runs), a capturing closure in tests.
func NewCmdClean(f *cmdutil.Factory, w wizard.Wizard, runF func(*CleanOptions) error) *cobra.Command {
	opts := &CleanOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
		Progress: f.Progress,
		Wizard:   w,
		Config:   f.Config,
	}

	cmd := &cobra.Command{
		Use:   domain.CmdClean + " [branch]",
		Short: "Remove a worktree and its local branch",
		Long: "Remove a git worktree and delete the local branch. The remote branch is never touched.\n" +
			"Without arguments, shows an interactive picker.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Branch = args[0]
			}
			if err := completeOptions(cmd, opts); err != nil {
				return err
			}
			if err := validateOptions(opts); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return cleanRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Force, domain.FlagForce, false,
		"Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes")
	cmd.Flags().BoolVarP(&opts.Yes, domain.FlagYes, "y", false,
		"Skip all prompts and run non-interactively; resolve every decision from flags and safe defaults (requires a branch; keeps safety checks unless --force)")
	cmd.Flags().BoolVar(&opts.ReparentChildren, domain.FlagReparentChildren, false,
		"Reparent orphaned child worktrees onto the grandparent (no prompt)")
	cmd.Flags().BoolVar(&opts.DryRun, domain.FlagDryRun, false,
		"Preview what would be removed without deleting anything (no confirmation, no --yes needed)")
	shared.AddOutputFlag(cmd)

	return cmd
}

// completeOptions resolves interactivity and loads config once. It does NOT prompt
// field-by-field: interactive collection is the composite wizard's job (cleanRun).
func completeOptions(cmd *cobra.Command, opts *CleanOptions) error {
	opts.Output, _ = cmd.Flags().GetString(domain.FlagOutput)

	// The --yes fold: --yes turns off the whole interactive mode (collection AND
	// confirmation), not just the [y/N]. So `clean --yes` with no branch errors
	// instead of opening the picker.
	opts.Interactive = opts.IO.CanPrompt() && !opts.Yes

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

// validateOptions is the safety net common to both modes.
func validateOptions(opts *CleanOptions) error {
	if opts.Output == domain.OutputJSON && !opts.Yes && !opts.DryRun {
		return cmdutil.FlagErrorf("--output json requires --yes (confirmations cannot run in JSON mode) or --dry-run")
	}
	// The picker is the only way to select a branch without naming it, and it runs
	// ONLY in a real interactive collection (a TTY, not --yes AND not --dry-run —
	// dry-run never opens the picker). Every other path, dry-run included, must name
	// the worktree explicitly.
	interactiveCollection := opts.Interactive && !opts.DryRun
	if opts.Branch == "" && !interactiveCollection {
		return domain.ErrCleanBranchRequired
	}
	return nil
}
