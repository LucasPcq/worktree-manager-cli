package inittui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunGlobalWizard presents the global config setup wizard.
// Returns ErrUserAborted if the user presses Esc at the first step.
func RunGlobalWizard() (domain.InitGlobalAnswers, error) {
	steps := []components.Step{
		{
			Name: "AI agent",
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Default AI agent",
				Description: "Which agent do you use for development?",
				Items: []components.SelectItem{
					{Label: "Claude Code", Value: string(domain.AgentClaudeCode)},
					{Label: "Cursor", Value: string(domain.AgentCursor)},
					{Label: "None", Value: string(domain.AgentNone)},
				},
			}),
			Summary: func(model any) string {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return ""
				}
				return sl.Value()
			},
		},
		{
			Name: "Shell",
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Shell",
				Description: "Your primary shell (for shell-init integration)",
				Items: []components.SelectItem{
					{Label: "zsh", Value: string(domain.ShellZsh)},
					{Label: "bash", Value: string(domain.ShellBash)},
					{Label: "fish", Value: string(domain.ShellFish)},
				},
			}),
			Summary: func(model any) string {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return ""
				}
				return sl.Value()
			},
		},
	}

	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.InitGlobalAnswers{}, fmt.Errorf("global wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}
	if final.Aborted() {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}

	agentStep, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}

	shellStep, ok := final.Steps()[1].Model.(components.SelectListModel)
	if !ok {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}

	return domain.InitGlobalAnswers{
		Agent: domain.AgentType(agentStep.Value()),
		Shell: domain.ShellType(shellStep.Value()),
	}, nil
}
