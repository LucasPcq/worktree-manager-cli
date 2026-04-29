package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run command group — manages dev jobs
// (services + one-shot tasks) declared in .wtm/run.toml.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdRun,
		Short:   "Manage dev jobs (services + tasks)",
		Long:    "Run commands and profiles declared in .wtm/run.toml — long-running services and one-shot tasks.",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newPsCmd())
	cmd.AddCommand(newUpCmd())
	cmd.AddCommand(newDownCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newImportCmd())

	return cmd
}
