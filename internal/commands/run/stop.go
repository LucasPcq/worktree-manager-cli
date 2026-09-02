package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newStopCmd creates the wtm run stop subcommand.
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStop + " [worktree]",
		Short: "Stop a single job",
		Long:  "Stop one running job of [worktree] — the current one when omitted, picked interactively when there is a terminal.\nThe job is named with --job; without it, a fully interactive run offers a picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runStop,
	}
	shared.AddJobFlag(cmd, "Job to stop (required without a terminal or in --output json mode)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
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
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := stopflow.Run(stopflow.Params{
		Context: shared.FlowContext(result),
		Request: stopflow.Request{
			Worktree: firstArg(args),
			Cwd:      dir,
			Job:      job,
			Config:   runCfg,
		},
		Prompter: shared.FlowPrompter(shared.FlowPrompterParams{
			Interactive: isTTY() && rules.IsHumanFormat(format),
			Stderr:      true,
		}),
		Presenter: stopPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}
