// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/agents"
	"github.com/LucasPcq/wtm/internal/commands/checkout"
	"github.com/LucasPcq/wtm/internal/commands/configcmd"
	"github.com/LucasPcq/wtm/internal/commands/daemon"
	"github.com/LucasPcq/wtm/internal/commands/initcmd"
	"github.com/LucasPcq/wtm/internal/commands/resolve"
	"github.com/LucasPcq/wtm/internal/commands/run"
	"github.com/LucasPcq/wtm/internal/commands/schema"
	"github.com/LucasPcq/wtm/internal/commands/shell"
	"github.com/LucasPcq/wtm/internal/commands/wt"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: domain.CmdGroupCore, Title: "Core Commands:"},
		&cobra.Group{ID: domain.CmdGroupSetup, Title: "Setup:"},
	)

	for _, cmd := range wt.NewCmds() {
		rootCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(run.NewCmd())

	checkoutCmd := checkout.NewCmd()
	checkoutCmd.GroupID = domain.CmdGroupCore
	rootCmd.AddCommand(checkoutCmd)

	initCmd := initcmd.NewCmd()
	initCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(initCmd)

	rootCmd.AddCommand(configcmd.NewCmd())

	shellInitCmd := shell.NewCmd()
	shellInitCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(shellInitCmd)

	rootCmd.AddCommand(schema.NewCmd())
	rootCmd.AddCommand(agents.NewCmd())
	rootCmd.AddCommand(resolve.NewCmd())
	rootCmd.AddCommand(daemon.NewCmd())
}

var version = domain.Version

var rootCmd = &cobra.Command{
	Use:           domain.AppName,
	Short:         "Orchestrate git worktrees and team dev workflows from the terminal",
	Version:       version,
	RunE:          rootRunE,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func rootRunE(cmd *cobra.Command, _ []string) error {
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
		// ErrAborted means the command already printed its own report; just
		// propagate the non-zero exit without a second error line.
		if !errors.Is(err, domain.ErrAborted) {
			output.Blank(os.Stderr)
			output.Error(os.Stderr, err.Error())
			output.Blank(os.Stderr)
		}
		os.Exit(rules.ExitCode(err))
	}
}
