package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CreateHooksPhaseParams holds inputs for running on_create hooks as a distinct,
// titled phase, shared by the worktree-creating commands that have not migrated
// to flow/ (extract, checkout) so hooks read the same way in both.
type CreateHooksPhaseParams struct {
	Cmd *cobra.Command
	// ShowHeader prints the "Running on_create hooks" title before the streamed
	// output; set it only on human-facing runs (never JSON).
	ShowHeader   bool
	ProjectDir   string
	StateDir     string
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
		StateDir:     p.StateDir,
		WorktreePath: p.WorktreePath,
		Branch:       p.Branch,
		FromBranch:   p.FromBranch,
		Hooks:        p.Hooks,
	})
}
