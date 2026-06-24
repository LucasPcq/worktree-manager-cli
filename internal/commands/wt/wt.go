package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmds returns the worktree subcommands, each assigned to the Core Commands
// help group, ready to be registered directly on the root command.
func NewCmds() []*cobra.Command {
	cmds := []*cobra.Command{
		newListCmd(),
		newCreateCmd(),
		newCleanCmd(),
		newSyncCmd(),
		newRelocateCmd(),
		newGoCmd(),
		newSwitchCmd(),
		newExtractCmd(),
	}

	for _, cmd := range cmds {
		cmd.GroupID = domain.CmdGroupCore
	}

	return cmds
}
