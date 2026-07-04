// Package bubbletea implements the progress.Runner capability with the shared
// bubbletea loading spinner. It is a tui adapter (importing components is expected
// here); the command layer depends only on progress.Runner.
package bubbletea

import (
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type runner struct {
	io *iostreams.IOStreams
}

// New returns a progress.Runner backed by the bubbletea spinner.
func New(io *iostreams.IOStreams) progress.Runner {
	return &runner{io: io}
}

// Run executes work behind the shared loading spinner.
func (r *runner) Run(params progress.RunParams, work func() error) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: params.Animate,
		Work:    work,
	})
}
