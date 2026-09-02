package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newUpCmd creates the wtm run up subcommand.
func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUp + " [worktree]",
		Short: "Start a profile's jobs",
		Long: "Start every job in a profile, in declared order, in [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"Without --profile, uses the default profile (or shows a picker if multiple exist).\n" +
			"Once the jobs are up, each declared port is checked: a port nothing answers on is\n" +
			"reported rather than announced as bound. It never fails the run — see --no-probe\n" +
			"and run.toml's port_probe_timeout.\n" +
			"Tasks block the profile and abort it on failure; services launch in the background.\n" +
			"When another worktree is already running jobs, wtm asks once what to do about it and can\n" +
			"remember the answer as run.toml's `concurrency`; --exclusive and --parallel override it\n" +
			"for one run.\n" +
			"The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.",
		Args: cobra.MaximumNArgs(1),
		RunE: runUp,
	}

	shared.AddProfileFlag(cmd, "Profile to start (defaults to the default profile, or a picker when several exist)")
	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.MarkFlagsMutuallyExclusive(domain.FlagExclusive, domain.FlagParallel)
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the jobs and return immediately instead of opening their output")
	cmd.Flags().Bool(domain.FlagNoProbe, false, "Skip the check that each declared port was actually bound")
	shared.AddYesFlag(cmd, "Skip all prompts; leaves the other worktrees' jobs running unless --exclusive")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}
	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return err
	}
	if err := reportRunConfig(cmd, runCfg); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	exclusive, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallel, _ := cmd.Flags().GetBool(domain.FlagParallel)
	noProbe, _ := cmd.Flags().GetBool(domain.FlagNoProbe)
	profile, _ := cmd.Flags().GetString(domain.FlagProfile)

	outcome, err := upflow.Run(upflow.Params{
		Context: shared.FlowContext(result),
		Request: upflow.Request{
			Worktree:  firstArg(args),
			Cwd:       dir,
			Profile:   profile,
			Exclusive: exclusive,
			Parallel:  parallel,
			NoProbe:   noProbe,
			Config:    runCfg,
		},
		// The run wizard may be reached through the shell wrapper, which
		// consumes stdout.
		Prompter: shared.FlowPrompter(shared.FlowPrompterParams{
			Interactive: shared.Interactive(shared.UnattendedParams{TTY: isTTY(), Format: format, Yes: yes}),
			Stderr:      true,
		}),
		Presenter: upPresenter{CLIPresenter: shared.NewPresenter(cmd, format), detach: detach},
	})
	if err != nil {
		return err
	}
	return concluded(outcome)
}

// reportRunConfig prints what the config got wrong before anything is started:
// warnings are advice, errors refuse the run.
func reportRunConfig(cmd *cobra.Command, cfg domain.RunConfig) error {
	warnings, errs := rules.ValidateRun(cfg)
	for _, warning := range warnings {
		output.Warning(cmd.ErrOrStderr(), warning)
	}
	if len(errs) == 0 {
		return nil
	}
	for _, e := range errs {
		output.Error(cmd.ErrOrStderr(), e)
	}
	return fmt.Errorf("invalid run config")
}
