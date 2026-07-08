// Package wizard is prune's interactive-flow port: a domain-only interface plus
// its DTOs. The command depends on this abstraction (never on tea/pruneui), and the
// tea adapter in internal/tui/pruneui satisfies it. Mirrors internal/cmd/clean/wizard.
package wizard

import "github.com/LucasPcq/wtm/internal/domain"

type Wizard interface {
	Run(PrunePrompt) (PruneChoice, error)
}

// PrunePrompt is the input the command hands the wizard. Force is the --force
// preset, threaded like clean's CleanPrompt.Force so a forced run pre-checks the
// unsafe candidates and the primary confirm already applies force (no re-ask).
type PrunePrompt struct {
	Plan            domain.PrunePlan
	Force           bool
	ReparentPreview func(chosen []string, force bool) []domain.ReparentResult
}

// PruneChoice is the wizard outcome. Aborted maps a user cancel (Esc / "No,
// cancel") to a value at the port boundary instead of an error.
type PruneChoice struct {
	Branches         []string
	Force            bool
	ReparentAsked    bool
	ReparentChildren bool
	Aborted          bool
}
