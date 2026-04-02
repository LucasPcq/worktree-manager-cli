package init

import (
	"errors"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RunGlobalWizard presents the global config setup form.
// Returns ErrUserAborted if the user presses Ctrl+C.
func RunGlobalWizard() (domain.InitGlobalAnswers, error) {
	var agent string
	var shell string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default AI agent").
				Description("Which agent do you use for development?").
				Options(
					huh.NewOption("Claude Code", string(domain.AgentClaudeCode)),
					huh.NewOption("Cursor", string(domain.AgentCursor)),
					huh.NewOption("None", string(domain.AgentNone)),
				).
				Value(&agent),

			huh.NewSelect[string]().
				Title("Shell").
				Description("Your primary shell (for shell-init integration)").
				Options(
					huh.NewOption("zsh", string(domain.ShellZsh)),
					huh.NewOption("bash", string(domain.ShellBash)),
					huh.NewOption("fish", string(domain.ShellFish)),
				).
				Value(&shell),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return domain.InitGlobalAnswers{}, domain.ErrUserAborted
		}
		return domain.InitGlobalAnswers{}, err
	}

	return domain.InitGlobalAnswers{
		Agent: domain.AgentType(agent),
		Shell: domain.ShellType(shell),
	}, nil
}
