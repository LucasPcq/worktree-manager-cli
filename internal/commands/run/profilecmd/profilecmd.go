// Package profilecmd implements `wtm run profile add|rm|edit` — CRUD on profiles in run.toml.
package profilecmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run profile command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdProfile,
		Short: "Add, remove, or edit profiles in run.toml",
		Long:  "Manage profiles declared in <git-common-dir>/wtm/run.toml.",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRmCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}
