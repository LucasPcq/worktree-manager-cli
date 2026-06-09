package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
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

	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		if format == domain.OutputJSON {
			return output.WriteJobResultsJSON(cmd.OutOrStdout(), nil)
		}
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "No jobs running.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	client := process.NewClient(socketPath)
	var stopSpinner func()
	if format != domain.OutputJSON {
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

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", args[0]))
	output.Blank(cmd.OutOrStdout())
	return nil
}
