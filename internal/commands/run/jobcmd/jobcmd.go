// Package jobcmd implements `wtm run job add|rm|edit` — CRUD on jobs in run.toml.
package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run job command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdJob,
		Short: "Add, remove, or edit jobs in run.toml",
		Long:  "Manage jobs declared in <git-common-dir>/wtm/run.toml.",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRmCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}
