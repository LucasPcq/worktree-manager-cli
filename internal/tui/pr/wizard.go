// Package pr builds interactive wizards for the wtm pr commands.
package pr

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// EnvPickerParams holds inputs for the PR checkout env strategy picker.
type EnvPickerParams struct {
	ConfigStrategy domain.EnvStrategy
}

// RunEnvPicker displays a single-question form asking for the env strategy.
// Returns the chosen strategy (empty string means "use config default").
func RunEnvPicker(params EnvPickerParams) (string, error) {
	items := []components.SelectItem{
		{Label: "Use config default (" + string(params.ConfigStrategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title:       "Env strategy",
		Description: "How to provision .env files in the new worktree",
		Items:       items,
	})

	result, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return "", domain.ErrUserAborted
	}
	return result, nil
}
