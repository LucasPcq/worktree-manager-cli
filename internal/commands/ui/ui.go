// Package ui implements `wtm ui`: the full-screen worktree dashboard.
package ui

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/dashboard"
)

// NewCmd creates the wtm ui command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUI,
		Short: "Open the worktree dashboard",
		Long: "Open a full-screen dashboard of the repository's worktrees.\n" +
			"The Worktrees tab lists them with their git state against both the base branch and\n" +
			"origin, and their pull requests; the Tree tab lays the same worktrees out as the\n" +
			"parent-child forest `wtm tree` prints. `n` creates a worktree; right-click a row\n" +
			"(or press `m`) to reparent or delete it; `a` opens the actions that run over\n" +
			"several worktrees at once. Local git state refreshes on a short poll; pull\n" +
			"requests load once and refresh only on `r`. Press `?` for the key reference.",
		Args: cobra.NoArgs,
		RunE: runUI,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runUI(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if !rules.IsHumanFormat(format) {
		return domain.ErrDashboardJSON
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return domain.ErrDashboardNotInteractive
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(dir)
	if err != nil {
		return err
	}

	return dashboard.Run(dashboard.RunParams{
		ProjectDir: result.ProjectDir,
		StateDir:   result.StateDir,
		Config:     result.Config,
		PRLoader:   func() ([]domain.PRInfo, domain.GHConnection) { return shared.LoadPRs(result.ProjectDir) },
	})
}
