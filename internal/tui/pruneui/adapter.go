package pruneui

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/cmd/prune/wizard"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type teaWizard struct {
	io *iostreams.IOStreams
}

func NewWizard(io *iostreams.IOStreams) wizard.Wizard {
	return &teaWizard{io: io}
}

func (w *teaWizard) Run(in wizard.PrunePrompt) (wizard.PruneChoice, error) {
	res, err := RunWizard(RunWizardParams{
		Plan:            in.Plan,
		Force:           in.Force,
		ReparentPreview: in.ReparentPreview,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return wizard.PruneChoice{Aborted: true}, nil
	}
	if err != nil {
		return wizard.PruneChoice{}, err
	}
	return wizard.PruneChoice{
		Branches:         res.Branches,
		Force:            res.Force,
		ReparentAsked:    res.ReparentAsked,
		ReparentChildren: res.ReparentChildren,
	}, nil
}
