// Package pr builds interactive wizards for the wtm pr commands.
package pr

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// CreateWizardParams holds inputs for the PR creation wizard.
type CreateWizardParams struct {
	Branches     []string
	DefaultBase  string
	AutoDraft    bool
	DefaultTitle string
}

// CreateWizardResult holds the answers from the PR creation wizard.
type CreateWizardResult struct {
	Title string
	Base  string
	Draft bool
}

// RunCreateWizard displays the interactive wizard for PR creation.
func RunCreateWizard(params CreateWizardParams) (CreateWizardResult, error) {
	titleInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "PR title",
		Description: "Title for the pull request",
		Placeholder: params.DefaultTitle,
	})

	branchItems := buildBranchItems(params.Branches, params.DefaultBase)
	branchList := components.NewSelectList(components.NewSelectListParams{
		Title:       "Base branch",
		Description: "Target branch for the PR",
		Items:       branchItems,
	})

	draftItems := buildDraftItems(params.AutoDraft)
	draftList := components.NewSelectList(components.NewSelectListParams{
		Title:       "Draft",
		Description: "",
		Items:       draftItems,
	})

	steps := []components.Step{
		{
			Name:  "PR title",
			Model: titleInput,
			Summary: func(model any) string {
				child, ok := model.(components.TextInputModel)
				if !ok {
					return ""
				}
				if v := child.Value(); v != "" {
					return v
				}
				if p := child.Placeholder(); p != "" {
					return p + " (default)"
				}
				return ""
			},
		},
		{
			Name:  "Base branch",
			Model: branchList,
			Summary: func(model any) string {
				if child, ok := model.(components.SelectListModel); ok {
					return child.Value()
				}
				return ""
			},
		},
		{
			Name:  "Draft",
			Model: draftList,
			Summary: func(model any) string {
				if child, ok := model.(components.SelectListModel); ok {
					return child.Value()
				}
				return ""
			},
		},
	}

	wiz := components.NewWizard(steps)
	p := tea.NewProgram(wiz)

	finalModel, err := p.Run()
	if err != nil {
		return CreateWizardResult{}, fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return CreateWizardResult{}, domain.ErrUserAborted
	}

	return extractResult(final.Steps()), nil
}

func extractResult(steps []components.Step) CreateWizardResult {
	var result CreateWizardResult

	if child, ok := steps[0].Model.(components.TextInputModel); ok {
		result.Title = child.Value()
		if result.Title == "" {
			result.Title = child.Placeholder()
		}
	}
	if child, ok := steps[1].Model.(components.SelectListModel); ok {
		result.Base = child.Value()
	}
	if child, ok := steps[2].Model.(components.SelectListModel); ok {
		result.Draft = child.Value() == "true"
	}

	return result
}

func buildBranchItems(branches []string, defaultBranch string) []components.SelectItem {
	var rest []string
	hasDefault := false
	for _, b := range branches {
		if b == defaultBranch {
			hasDefault = true
			continue
		}
		rest = append(rest, b)
	}

	sort.Strings(rest)

	items := make([]components.SelectItem, 0, len(branches))

	if hasDefault {
		items = append(items, components.SelectItem{
			Label: defaultBranch + " (default)",
			Value: defaultBranch,
		})
	}

	for _, b := range rest {
		items = append(items, components.SelectItem{
			Label: b,
			Value: b,
		})
	}

	return items
}

func buildDraftItems(autoDraft bool) []components.SelectItem {
	items := []components.SelectItem{
		{Label: "No — ready for review", Value: "false"},
		{Label: "Yes — work in progress", Value: "true"},
	}

	if autoDraft {
		items[0], items[1] = items[1], items[0]
	}

	return items
}

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
