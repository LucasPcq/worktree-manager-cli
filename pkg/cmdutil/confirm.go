package cmdutil

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type ConfirmParams struct {
	IO       *iostreams.IOStreams
	Prompter prompter.Prompter
	Yes bool
	Prompt string
}

func Confirm(p ConfirmParams) error {
	if p.Yes {
		return nil
	}
	if !p.IO.CanPrompt() {
		return FlagErrorf("confirmation required to proceed; re-run with --yes")
	}
	confirmed, err := p.Prompter.Confirm(p.Prompt, false)
	if err != nil {
		return err
	}
	if !confirmed {
		return domain.ErrUserAborted
	}
	return nil
}
