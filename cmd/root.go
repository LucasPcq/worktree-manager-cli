// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/cmd/clean"
	"github.com/LucasPcq/wtm/internal/cmd/prune"
	"github.com/LucasPcq/wtm/internal/commands/agents"
	"github.com/LucasPcq/wtm/internal/commands/checkout"
	"github.com/LucasPcq/wtm/internal/commands/configcmd"
	"github.com/LucasPcq/wtm/internal/commands/daemon"
	"github.com/LucasPcq/wtm/internal/commands/initcmd"
	"github.com/LucasPcq/wtm/internal/commands/resolve"
	"github.com/LucasPcq/wtm/internal/commands/run"
	"github.com/LucasPcq/wtm/internal/commands/schema"
	"github.com/LucasPcq/wtm/internal/commands/shell"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/commands/wt"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	progbubble "github.com/LucasPcq/wtm/internal/progress/bubbletea"
	pbubble "github.com/LucasPcq/wtm/internal/prompter/bubbletea"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/cleanui"
	"github.com/LucasPcq/wtm/internal/tui/pruneui"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
	"github.com/LucasPcq/wtm/pkg/iostreams"
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

	// clean is the Command-Factory pilot: built from a Factory (composition root),
	// with its interactive wizard injected (ports & adapters — the tea adapter lives
	// in internal/tui/cleanui). Registered here so the docs generator (which walks Root
	// without calling Execute) also sees it.
	cleanFactory := buildFactory()
	cleanCmd := clean.NewCmdClean(cleanFactory, cleanui.NewWizard(cleanFactory.IOStreams), nil)
	cleanCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(cleanCmd)

	// prune is likewise a Factory-pattern command with its interactive picker
	// injected as a ports & adapters wizard (the tea adapter lives in
	// internal/tui/pruneui). Registered here so the docs generator sees it.
	pruneFactory := buildFactory()
	pruneCmd := prune.NewCmdPrune(pruneFactory, pruneui.NewWizard(pruneFactory.IOStreams), nil)
	pruneCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(pruneCmd)

	rootCmd.AddCommand(run.NewCmd())

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

// buildFactory constructs the dependency container for Factory-based commands: real
// IOStreams, the bubbletea Prompter, and a cobra-free config loader.
func buildFactory() *cmdutil.Factory {
	io := iostreams.System()
	return &cmdutil.Factory{
		IOStreams: io,
		Prompter:  pbubble.New(io),
		Progress:  progbubble.New(io),
		Config: func(dir string) (shared.ConfigResult, error) {
			return shared.LoadConfigDir(dir)
		},
	}
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	cmd, err := rootCmd.ExecuteC()
	if err == nil {
		return
	}

	// A FlagError is a usage error: show the offending command's usage and exit
	// with the usage code (this is what makes cmdutil.FlagErrorf meaningful).
	var flagErr cmdutil.FlagError
	if errors.As(err, &flagErr) {
		output.Blank(os.Stderr)
		output.Error(os.Stderr, err.Error())
		fmt.Fprint(os.Stderr, cmd.UsageString())
		output.Blank(os.Stderr)
		os.Exit(domain.ExitCodeUsage)
	}

	// ErrAborted: the command already printed its own report. ErrUserAborted: a
	// clean user cancel — exit quietly, without an alarming error line.
	if !errors.Is(err, domain.ErrAborted) && !errors.Is(err, domain.ErrUserAborted) {
		output.Blank(os.Stderr)
		output.Error(os.Stderr, err.Error())
		output.Blank(os.Stderr)
	}
	os.Exit(rules.ExitCode(err))
}
