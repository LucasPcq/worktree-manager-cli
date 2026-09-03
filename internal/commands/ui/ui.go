// Package ui implements `wtm ui`: the full-screen worktree dashboard.
package ui

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
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
			"parent-child forest `wtm tree` prints; the Services tab gathers every worktree the\n" +
			"run daemon holds something up in, with the addresses its jobs answer on. `n`\n" +
			"creates a worktree; right-click a row (or press `m`) to reparent, sync, or delete\n" +
			"it; `a` opens the actions that run over several worktrees at once, syncing or\n" +
			"reparenting a selection of them; `L` reads a job's logs in the detail panel.\n" +
			"The list's local git state refreshes on a short poll; the detail panel reloads\n" +
			"when the selection changes or an operation touches it, and pull requests load\n" +
			"once — both refresh on demand with `r`.\n" +
			"Press `?` for the key reference.",
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

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	return dashboard.Run(buildRunParams(dir, result))
}

// buildRunParams assembles the dashboard's inputs from the resolved config and
// the directory `wtm ui` was actually launched from — split out from runUI so
// the wiring (Cwd in particular: the raw working directory, not ProjectDir,
// which LoadConfig may have resolved upward) is asserted directly rather than
// only reachable by running the dashboard itself, which needs a real terminal.
func buildRunParams(dir string, result shared.ConfigResult) dashboard.RunParams {
	return dashboard.RunParams{
		ProjectDir: result.ProjectDir,
		StateDir:   result.StateDir,
		Cwd:        infra.ResolvePath(dir),
		Config:     result.Config,
		PRLoader:   func() ([]domain.PRInfo, domain.GHConnection) { return shared.LoadPRsWithChecks(result.ProjectDir) },
		PROpener: func(number int) error {
			return ghservice.OpenPR(ghservice.OpenPRParams{ProjectDir: result.ProjectDir, Number: number})
		},
		URLOpener: integration.OpenURL,
		LogsLoader: dashboard.DefaultLogsLoader(dashboard.LogsLoaderParams{
			ProjectDir: result.ProjectDir,
			StateDir:   result.StateDir,
		}),
		// The public port is dialed here rather than once at startup: the loader
		// already runs off the UI goroutine, and a daemon started after the
		// dashboard was opened must not leave every address unpublished.
		AddressLoader: func(request dashboard.AddressRequest) map[string]map[string]domain.JobAddress {
			return worktree.RunAddressesFor(worktree.RunAddressesForParams{
				ProjectDir: result.ProjectDir,
				StateDir:   result.StateDir,
				RunConfig:  request.Config,
				Branches:   request.Branches,
				ProxyPort:  process.PublicProxyPort(rules.ProxyPort(result.Config.Global)),
			})
		},
	}
}
