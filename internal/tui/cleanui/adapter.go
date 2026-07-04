package cleanui

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/cmd/clean/wizard"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

type teaWizard struct {
	io *iostreams.IOStreams
}

func NewWizard(io *iostreams.IOStreams) wizard.Wizard {
	return &teaWizard{io: io}
}

func (w *teaWizard) Run(in wizard.CleanPrompt) (wizard.CleanChoice, error) {
	res, err := RunWizard(RunWizardParams{
		ProjectDir:        in.ProjectDir,
		PreselectedBranch: in.PreselectedBranch,
		PreCheck:          in.PreCheck,
		Force:             in.Force,
		ReparentChildren:  in.ReparentChildren,
		Check:             in.Check,
		ReparentPreview:   in.ReparentPreview,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return wizard.CleanChoice{Aborted: true}, nil
	}
	if err != nil {
		return wizard.CleanChoice{}, err
	}
	return wizard.CleanChoice{
		Branch:           res.Branch,
		Force:            res.Force,
		ReparentAsked:    res.ReparentAsked,
		ReparentChildren: res.ReparentChildren,
	}, nil
}
