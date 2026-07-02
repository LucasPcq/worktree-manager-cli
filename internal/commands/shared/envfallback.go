package shared

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
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
func EnvParentFallbackApplies(params EnvFallbackParams) bool {
	return worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	})
}

// EnvParentFallbackConfirm builds the confirmation prompt shown when the parent
// env strategy falls back to main for the given source branch.
func EnvParentFallbackConfirm(source string) components.NewConfirmParams {
	return components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.EnvParentFallbackPrompt, source),
		Warning:    domain.EnvParentFallbackWarning,
		DefaultYes: true,
	}
}

// EnvFallbackDecider returns a decider for a wizard ConfirmStep: given a resolved
// source branch and env override it reports whether the parent-strategy fallback
// confirmation applies and, if so, the prompt to show. Shared by every create-like
// flow (create, checkout, extract) so the confirmation lives inside their wizard.
func EnvFallbackDecider(projectDir string, config domain.Config) func(source, envOverride string) (bool, components.NewConfirmParams) {
	return func(source, envOverride string) (bool, components.NewConfirmParams) {
		if !EnvParentFallbackApplies(EnvFallbackParams{
			ProjectDir:  projectDir,
			Source:      source,
			Config:      config,
			EnvOverride: envOverride,
		}) {
			return false, components.NewConfirmParams{}
		}
		return true, EnvParentFallbackConfirm(source)
	}
}
