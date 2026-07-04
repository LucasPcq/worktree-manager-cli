package cmdutil

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type Factory struct {
	IOStreams *iostreams.IOStreams
	Prompter  prompter.Prompter
	Progress  progress.Runner
	Config    func(dir string) (shared.ConfigResult, error)
}

type FlagError struct{ err error }

func (fe FlagError) Error() string { return fe.err.Error() }
func (fe FlagError) Unwrap() error { return fe.err }

func FlagErrorf(format string, a ...any) error {
	return FlagError{err: fmt.Errorf(format, a...)}
}
