package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newStartCmd creates the wtm run start subcommand.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStart + " [worktree]",
		Short: "Start a single job",
		Long: "Start one job of [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"The job is named with --job; without it, a fully interactive run offers a picker.\n" +
			"A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.\n" +
			"-d starts it and returns the prompt instead.\n" +
			"A task always runs inline and blocks until it exits, with or without -d.",
		Args: cobra.MaximumNArgs(1),
		RunE: runStart,
	}
	shared.AddJobFlag(cmd, "Job to start (required without a terminal or in --output json mode)")
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the service and return immediately instead of opening its output")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := startflow.Run(startflow.Params{
		Context: shared.FlowContext(result),
		Request: startflow.Request{
			Worktree: firstArg(args),
			Cwd:      dir,
			Job:      job,
			Config:   runCfg,
		},
		Prompter: shared.FlowPrompter(shared.FlowPrompterParams{
			Interactive: isTTY() && rules.IsHumanFormat(format),
			Stderr:      true,
		}),
		Presenter: startPresenter{CLIPresenter: shared.NewPresenter(cmd, format), detach: detach},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}
