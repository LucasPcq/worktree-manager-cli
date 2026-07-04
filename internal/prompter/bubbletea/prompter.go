// Package bubbletea is the concrete implementation of the generic input Prompter.
// Output feedback (spinners) lives in internal/progress; command-specific wizards
// live behind their own port + adapter (see internal/tui/cleanui).
package bubbletea

import (
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type bubbleteaPrompter struct {
	io *iostreams.IOStreams
}

// New returns a Prompter backed by the bubbletea TUI.
func New(io *iostreams.IOStreams) prompter.Prompter {
	return &bubbleteaPrompter{io: io}
}

// Confirm renders a standalone [y/N] dialog. Destructive callers pass
// defaultYes=false so Enter cancels.
func (p *bubbleteaPrompter) Confirm(prompt string, defaultYes bool) (bool, error) {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      prompt,
		DefaultYes: defaultYes,
	})
	return components.RunStandaloneConfirm(cm)
}
