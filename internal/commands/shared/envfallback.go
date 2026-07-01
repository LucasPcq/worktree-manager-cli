package shared

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// EnvFallbackParams holds inputs for ConfirmEnvParentFallback.
type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// ConfirmEnvParentFallback warns, before a worktree is created, when the "parent"
// env strategy will silently source .env from the main worktree because the source
// branch has no local worktree. It returns true to proceed (no fallback, or the
// user confirmed) and false when the user declines. Interactive only — callers
// should skip it in non-interactive mode.
func ConfirmEnvParentFallback(params EnvFallbackParams) bool {
	if !worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	}) {
		return true
	}

	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("%s has no local worktree — copy .env from the main worktree instead of the parent?", params.Source),
		Warning:    "The \"parent\" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.",
		DefaultYes: true,
	}))
	return confirmed
}
