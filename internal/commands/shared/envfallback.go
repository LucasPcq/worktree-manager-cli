package shared

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// EnvFallbackParams holds inputs for EnvParentFallbackApplies.
type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// EnvParentFallbackApplies reports whether the "parent" env strategy will
// silently source .env from the main worktree because the source branch has no
// local worktree. Callers gate a confirmation on it — an in-wizard ConfirmStep on
// the interactive paths, so declining goes back rather than aborting outright.
//
// This is a pure boundary check (domain + service only). The tea-typed prompt it
// feeds lives in internal/tui/envconfirm so this package stays tea-free — the
// architecture test in internal/archtest depends on it.
func EnvParentFallbackApplies(params EnvFallbackParams) bool {
	return worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	})
}
