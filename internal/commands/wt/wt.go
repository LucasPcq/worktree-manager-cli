package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmds returns the worktree subcommands, each assigned to its root --help
// group, ready to be registered directly on the root command.
func NewCmds() []*cobra.Command {
	grouped := func(cmd *cobra.Command, group string) *cobra.Command {
		cmd.GroupID = group
		return cmd
	}

	return []*cobra.Command{
		grouped(newListCmd(), domain.CmdGroupWorktrees),
		grouped(newTreeCmd(), domain.CmdGroupWorktrees),
		grouped(newCreateCmd(), domain.CmdGroupWorktrees),
		// clean and prune are migrated to the Factory pattern and registered from
		// cmd/root.go (see internal/cmd/clean and internal/cmd/prune).
		grouped(newExtractCmd(), domain.CmdGroupWorktrees),
		grouped(newRelocateCmd(), domain.CmdGroupWorktrees),
		grouped(newGoCmd(), domain.CmdGroupNavigate),
		grouped(newSwitchCmd(), domain.CmdGroupNavigate),
		grouped(newSyncCmd(), domain.CmdGroupStack),
		grouped(newReparentCmd(), domain.CmdGroupStack),
	}
}
