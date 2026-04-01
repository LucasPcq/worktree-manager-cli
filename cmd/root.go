// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"os"

	"github.com/LucasPcq/wtm/internal/commands"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(commands.NewInitCmd())
	rootCmd.AddCommand(commands.NewNewCmd())
	rootCmd.AddCommand(commands.NewLsCmd())
	rootCmd.AddCommand(commands.NewCleanCmd())
}

var version = domain.Version

var rootCmd = &cobra.Command{
	Use:     domain.AppName,
	Short:   "Worktree Manager — orchestrate git worktrees, AI agents, and team workflows",
	Version: version,
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(domain.ExitCodeError)
	}
}
