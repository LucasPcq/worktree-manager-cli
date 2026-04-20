package commands

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewRunCmd creates the wtm run command group — manages dev jobs
// (services + one-shot tasks) declared in .wtm/run.toml.
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdRun,
		Short:   "Manage dev jobs (services + tasks)",
		Long:    "Run commands and profiles declared in .wtm/run.toml — long-running services and one-shot tasks.",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newRunListCmd())
	cmd.AddCommand(newRunPsCmd())
	cmd.AddCommand(newRunUpCmd())
	cmd.AddCommand(newRunDownCmd())
	cmd.AddCommand(newRunStartCmd())
	cmd.AddCommand(newRunStopCmd())
	cmd.AddCommand(newRunLogsCmd())
	cmd.AddCommand(newRunExportCmd())
	cmd.AddCommand(newRunImportCmd())

	return cmd
}
