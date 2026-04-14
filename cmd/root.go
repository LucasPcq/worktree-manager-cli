// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"os"
	"strings"

	"github.com/LucasPcq/wtm/internal/commands"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: domain.CmdGroupCore, Title: "Core Commands:"},
		&cobra.Group{ID: domain.CmdGroupSetup, Title: "Setup:"},
	)

	rootCmd.AddCommand(commands.NewWtCmd())
	rootCmd.AddCommand(commands.NewRunCmd())
	rootCmd.AddCommand(commands.NewPRCmd())

	initCmd := commands.NewInitCmd()
	initCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(initCmd)

	shellInitCmd := commands.NewShellInitCmd()
	shellInitCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(shellInitCmd)

	rootCmd.AddCommand(commands.NewSchemaCmd())

	rootCmd.AddCommand(commands.NewAgentsCmd())

	rootCmd.AddCommand(commands.NewResolveCmd())
	if domain.FeatureDashboard {
		rootCmd.AddCommand(commands.NewDashboardCmd())
	}
	rootCmd.AddCommand(commands.NewDaemonCmd())
}

var version = domain.Version

var rootCmd = &cobra.Command{
	Use:           domain.AppName,
	Short:         "Worktree Manager — orchestrate git worktrees, AI agents, and team workflows",
	Version:       version,
	RunE:          rootRunE,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func rootRunE(cmd *cobra.Command, args []string) error {
	if domain.FeatureDashboard {
		return commands.RunDashboard(cmd, args)
	}
	return cmd.Help()
}

func init() {
	// Override the global help function to add consistent padding around help text.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		w := cmd.OutOrStdout()
		output.Blank(w)

		// Temporarily swap the output writer to capture and indent the help text.
		orig := cmd.OutOrStdout()
		var buf strings.Builder
		cmd.SetOut(&buf)
		defaultHelp(cmd, args)
		cmd.SetOut(orig)

		for _, line := range strings.Split(buf.String(), "\n") {
			if line == "" {
				output.Blank(w)
			} else {
				output.Message(w, line)
			}
		}
	})
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Blank(os.Stderr)
		output.Error(os.Stderr, err.Error())
		output.Blank(os.Stderr)
		os.Exit(domain.ExitCodeError)
	}
}
