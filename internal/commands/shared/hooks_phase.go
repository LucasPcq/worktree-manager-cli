package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CreateHooksPhaseParams holds inputs for running on_create hooks as a distinct,
// titled phase, shared by every worktree-creating command (create, extract,
// checkout) so hooks read the same way everywhere.
type CreateHooksPhaseParams struct {
	Cmd *cobra.Command
	// ShowHeader prints the "Running on_create hooks" title before the streamed
	// output; set it only on human-facing runs (never JSON).
	ShowHeader   bool
	ProjectDir   string
	WorktreePath string
	Branch       string
	FromBranch   string
	Hooks        []domain.HookCommand
}

// RunCreateHooksPhase runs the on_create hooks under a titled section. No-op when
// no hooks are configured.
func RunCreateHooksPhase(p CreateHooksPhaseParams) error {
	if len(p.Hooks) == 0 {
		return nil
	}
	if p.ShowHeader {
		output.HooksSection(p.Cmd.ErrOrStderr(), domain.HooksTitleOnCreate)
	}
	return worktree.RunCreateHooks(domain.CreateHooksParams{
		ProjectDir:   p.ProjectDir,
		WorktreePath: p.WorktreePath,
		Branch:       p.Branch,
		FromBranch:   p.FromBranch,
		Hooks:        p.Hooks,
	})
}

// CleanHooksPhaseParams holds inputs for running on_clean hooks as a distinct,
// titled phase, shared by the worktree-removing commands (clean, prune).
type CleanHooksPhaseParams struct {
	Cmd          *cobra.Command
	ShowHeader   bool
	ProjectDir   string
	WorktreePath string
	Branch       string
	Hooks        []domain.HookCommand
}

// RunCleanHooksPhase runs the on_clean hooks under a titled section. No-op when no
// hooks are configured or the worktree path is unknown.
func RunCleanHooksPhase(p CleanHooksPhaseParams) error {
	if len(p.Hooks) == 0 || p.WorktreePath == "" {
		return nil
	}
	if p.ShowHeader {
		output.HooksSection(p.Cmd.ErrOrStderr(), domain.HooksTitleOnClean)
	}
	return worktree.RunCleanHooks(domain.CleanHooksParams{
		ProjectDir:   p.ProjectDir,
		WorktreePath: p.WorktreePath,
		Branch:       p.Branch,
		Hooks:        p.Hooks,
	})
}
