package resolve

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// NewCmd creates the wtm resolve command (internal, used by shell wrapper).
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "resolve [branch]",
		Short:  "Resolve a branch to its worktree path",
		Hidden: true,
		RunE:   runResolve,
	}
}

func runResolve(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	root, err := shared.ProjectRoot(dir)
	if err != nil {
		return err
	}

	query := strings.Join(args, " ")

	result, err := worktree.Resolve(domain.ResolveParams{
		ProjectDir: root,
		Query:      query,
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("No worktree found matching %q", query))
		output.Blank(cmd.ErrOrStderr())
		return nil
	}
	if err != nil {
		return err
	}

	if !result.Ambiguous {
		fmt.Fprintln(cmd.OutOrStdout(), result.Path)
		return nil
	}

	selected, pickErr := pickAmbiguousWorktree(cmd, root, result.Matches)
	if errors.Is(pickErr, domain.ErrUserAborted) {
		return nil
	}
	if pickErr != nil {
		return pickErr
	}

	fmt.Fprintln(cmd.OutOrStdout(), selected.Path)
	return nil
}

func pickAmbiguousWorktree(cmd *cobra.Command, projectDir string, matches []domain.GitWorktree) (domain.WorktreeStatus, error) {
	cfgResult, err := shared.LoadConfig(cmd, projectDir)
	if err != nil {
		return domain.WorktreeStatus{}, err
	}

	// Load worktree statuses and running services (both fast/local) in parallel
	// behind a single spinner. PRs (the slow gh call) are fetched lazily inside
	// the picker so the list shows instantly. Everything goes to stderr: stdout
	// is reserved for the resolved path the shell wrapper reads.
	var (
		statuses []domain.WorktreeStatus
		listErr  error
		services []domain.JobInfo
		wg       sync.WaitGroup
	)

	stop := shared.StartSpinner(cmd.ErrOrStderr(), "Loading worktrees…")
	wg.Add(2)
	go func() {
		defer wg.Done()
		statuses, listErr = worktree.List(domain.ListParams{
			ProjectDir: cfgResult.ProjectDir,
			StateDir:   cfgResult.StateDir,
			Config:     cfgResult.Config,
		})
	}()
	go func() {
		defer wg.Done()
		services = shared.LoadJobsGraceful()
	}()
	wg.Wait()
	stop()

	if listErr != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("list worktrees: %w", listErr)
	}

	filtered := rules.FilterStatusesByMatches(statuses, matches)
	if len(filtered) == 0 {
		filtered = statuses
	}

	return worktreepicker.Run(worktreepicker.RunParams{
		Statuses: filtered,
		Services: services,
		Title:    "Select a worktree",
		PRLoader: func() ([]domain.PRInfo, domain.GHConnection) {
			return shared.LoadPRs(cfgResult.ProjectDir)
		},
	})
}
