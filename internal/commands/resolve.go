package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// NewResolveCmd creates the wtm resolve command (internal, used by shell wrapper).
func NewResolveCmd() *cobra.Command {
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

	root, err := projectRoot(dir)
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

// pickAmbiguousWorktree shows the shared worktree picker, filtered to the
// branches that matched the user's query.
func pickAmbiguousWorktree(cmd *cobra.Command, projectDir string, matches []domain.GitWorktree) (domain.WorktreeStatus, error) {
	cfgResult, ok := loadConfig(cmd, projectDir)
	if !ok {
		return domain.WorktreeStatus{}, domain.ErrUserAborted
	}

	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: cfgResult.ProjectDir,
		Config:     cfgResult.Config,
	})
	if err != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("list worktrees: %w", err)
	}

	filtered := filterStatusesByMatches(statuses, matches)
	if len(filtered) == 0 {
		filtered = statuses
	}

	prs := loadPRsGraceful(cfgResult.ProjectDir)
	services := loadJobsGraceful()

	return worktreepicker.Run(worktreepicker.RunParams{
		Statuses: filtered,
		PRs:      prs,
		Services: services,
		Title:    "Select a worktree",
	})
}

func filterStatusesByMatches(statuses []domain.WorktreeStatus, matches []domain.GitWorktree) []domain.WorktreeStatus {
	if len(matches) == 0 {
		return statuses
	}
	allowed := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		allowed[m.Branch] = struct{}{}
	}
	out := make([]domain.WorktreeStatus, 0, len(matches))
	for _, s := range statuses {
		if _, ok := allowed[s.Branch]; ok {
			out = append(out, s)
		}
	}
	return out
}
