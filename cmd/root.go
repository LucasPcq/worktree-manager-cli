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
	"github.com/LucasPcq/wtm/internal/commands/ui"
	"github.com/LucasPcq/wtm/internal/commands/upgrade"
	"github.com/LucasPcq/wtm/internal/commands/wt"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: domain.CmdGroupWorktrees, Title: domain.CmdGroupWorktreesTitle},
		&cobra.Group{ID: domain.CmdGroupNavigate, Title: domain.CmdGroupNavigateTitle},
		&cobra.Group{ID: domain.CmdGroupStack, Title: domain.CmdGroupStackTitle},
		&cobra.Group{ID: domain.CmdGroupJobs, Title: domain.CmdGroupJobsTitle},
		&cobra.Group{ID: domain.CmdGroupGitHub, Title: domain.CmdGroupGitHubTitle},
		&cobra.Group{ID: domain.CmdGroupSetup, Title: domain.CmdGroupSetupTitle},
	)

	for _, cmd := range wt.NewCmds() {
		rootCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(run.NewCmd())

	uiCmd := ui.NewCmd()
	uiCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(uiCmd)

	resolveCmd := resolve.NewCmd()
	resolveCmd.GroupID = domain.CmdGroupNavigate
	rootCmd.AddCommand(resolveCmd)

	checkoutCmd := checkout.NewCmd()
	checkoutCmd.GroupID = domain.CmdGroupGitHub
	rootCmd.AddCommand(checkoutCmd)

	initCmd := initcmd.NewCmd()
	initCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(initCmd)

	shellInitCmd := shell.NewCmd()
	shellInitCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(shellInitCmd)

	configCmd := configcmd.NewCmd()
	configCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(configCmd)

	agentsCmd := agents.NewCmd()
	agentsCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(agentsCmd)

	schemaCmd := schema.NewCmd()
	schemaCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(schemaCmd)

	upgradeCmd := upgrade.NewCmd(upgrade.NewCmdParams{Version: version})
	upgradeCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(upgradeCmd)

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

// Root returns the fully assembled root command. It exists so tooling (e.g. the
// docs generator under tools/gendocs) can walk the command tree without invoking it.
func Root() *cobra.Command {
	return rootCmd
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
