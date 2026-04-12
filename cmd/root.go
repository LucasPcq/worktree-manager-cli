// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"os"

	"github.com/LucasPcq/wtm/internal/commands"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: domain.CmdGroupCore, Title: "Core Commands:"},
		&cobra.Group{ID: domain.CmdGroupSetup, Title: "Setup:"},
	)

	rootCmd.AddCommand(commands.NewWtCmd())
	rootCmd.AddCommand(commands.NewSvcCmd())
	rootCmd.AddCommand(commands.NewPRCmd())

	initCmd := commands.NewInitCmd()
	initCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(initCmd)

	shellInitCmd := commands.NewShellInitCmd()
	shellInitCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(shellInitCmd)

	rootCmd.AddCommand(commands.NewResolveCmd())
	if domain.FeatureDashboard {
		rootCmd.AddCommand(commands.NewDashboardCmd())
	}
	rootCmd.AddCommand(commands.NewDaemonCmd())
}

var version = domain.Version

var rootCmd = &cobra.Command{
	Use:     domain.AppName,
	Short:   "Worktree Manager — orchestrate git worktrees, AI agents, and team workflows",
	Version: version,
	RunE:    rootRunE,
}

func rootRunE(cmd *cobra.Command, args []string) error {
	if domain.FeatureDashboard {
		return commands.RunDashboard(cmd, args)
	}
	return cmd.Help()
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(domain.ExitCodeError)
	}
}
