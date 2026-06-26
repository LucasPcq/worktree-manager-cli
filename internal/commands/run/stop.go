package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newStopCmd creates the wtm run stop subcommand.
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStop + " <job>",
		Short: "Stop a single job",
		Long:  "Stop an individual running job by name.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStop,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	// Validate the job is declared so a typo'd name fails with a precise exit
	// code instead of silently no-opping at the daemon.
	stateDir, err := shared.StateDir(dir)
	if err != nil {
		return err
	}
	runCfg, err := config.LoadRun(stateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}
	if _, declared := rules.FindJob(runCfg, args[0]); !declared {
		return fmt.Errorf("%w: %s", domain.ErrJobNotFound, args[0])
	}

	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		if format == domain.OutputJSON {
			return output.WriteJobResultsJSON(cmd.OutOrStdout(), nil)
		}
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No jobs running.")
		})
		return nil
	}

	client := process.NewClient(socketPath)
	var stopSpinner func()
	if rules.IsHumanFormat(format) {
		stopSpinner = shared.StartSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Stopping %s…", args[0]))
	}
	resp, err := client.Send(process.Request{
		Action:  process.ActionStop,
		Name:    args[0],
		WorkDir: dir,
	})
	if stopSpinner != nil {
		stopSpinner()
	}
	if err != nil {
		return fmt.Errorf("stop %s: %w", args[0], err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop %s: %s", args[0], resp.Message)
	}

	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(cmd.OutOrStdout(), output.JobActionResult{
			Name:   args[0],
			Status: domain.JobActionStopped,
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", args[0]))
	})
	return nil
}
