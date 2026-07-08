// Package envconfirm builds the "parent env strategy falls back to main"
// confirmation for the create-like flows (create, checkout, extract). It returns a
// components.NewConfirmParams, so it lives in the tui layer — keeping this tea-typed
// helper out of internal/commands/shared, which must stay tea-free (see the
// architecture test in internal/archtest). The pure boundary check it builds on,
// shared.EnvParentFallbackApplies, stays in shared.
package envconfirm

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Confirm builds the confirmation prompt shown when the parent env strategy falls
// back to main for the given source branch.
func Confirm(source string) components.NewConfirmParams {
	return components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.EnvParentFallbackPrompt, source),
		Warning:    domain.EnvParentFallbackWarning,
		DefaultYes: true,
	}
}

// Decider returns a decider for a wizard ConfirmStep: given a resolved source
// branch and env override it reports whether the parent-strategy fallback
// confirmation applies and, if so, the prompt to show. Shared by every create-like
// flow (create, checkout, extract) so the confirmation lives inside their wizard.
func Decider(projectDir string, config domain.Config) func(source, envOverride string) (bool, components.NewConfirmParams) {
	return func(source, envOverride string) (bool, components.NewConfirmParams) {
		if !shared.EnvParentFallbackApplies(shared.EnvFallbackParams{
			ProjectDir:  projectDir,
			Source:      source,
			Config:      config,
			EnvOverride: envOverride,
		}) {
			return false, components.NewConfirmParams{}
		}
		return true, Confirm(source)
	}
}
