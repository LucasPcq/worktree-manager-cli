package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

// newSwitchCmd creates the wtm switch subcommand.
func newSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdSwitch + " [branch]",
		Short: "Navigate to a worktree and start its services",
		Long:  "Combines `go` and `svc up` in one command.\nRequires shell integration to change your working directory.",
		RunE:  runSwitch,
	}

	cmd.Flags().String(domain.FlagProfile, "", "Service profile to start")
	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop services on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")

	return cmd
}

func runSwitch(cmd *cobra.Command, _ []string) error {
	output.Blank(cmd.ErrOrStderr())
	output.Warning(cmd.ErrOrStderr(), "wtm switch requires shell integration to change your working directory.")
	output.Blank(cmd.ErrOrStderr())
	output.Message(cmd.ErrOrStderr(), domain.MsgShellInitHint)
	output.Blank(cmd.ErrOrStderr())
	return nil
}
