package wizard

import "github.com/LucasPcq/wtm/internal/domain"

type Wizard interface {
	Run(CleanPrompt) (CleanChoice, error)
}

type CleanPrompt struct {
	ProjectDir        string
	PreselectedBranch string
	PreCheck          *domain.CleanCheckResult
	Force             bool
	ReparentChildren  bool
	Check             func(branch string) domain.CleanCheckResult
	ReparentPreview   func(branch string) domain.CleanReparentPlan
}

type CleanChoice struct {
	Branch           string
	Force            bool
	ReparentAsked    bool
	ReparentChildren bool
	Aborted          bool
}
