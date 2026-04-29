// Package newwt builds interactive wizards for the wtm new command.
package newwt

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// WizardParams holds inputs for the interactive new worktree wizard.
type WizardParams struct {
	Branches       []string
	DefaultBranch  string
	ConfigStrategy domain.EnvStrategy
	IncludeBranch  bool
}

// WizardResult holds the answers from the interactive wizard.
type WizardResult struct {
	BranchName      string
	FromBranch      string
	EnvFromOverride string
}

// RunWizard displays the interactive wizard for wtm new.
// When IncludeBranch is true, the first step prompts for a branch name.
// Returns ErrUserAborted on Ctrl+C or Esc at the first step.
func RunWizard(params WizardParams) (WizardResult, error) {
	var steps []components.Step

	if params.IncludeBranch {
		branchInput := components.NewTextInput(components.NewTextInputParams{
			Title:       "Branch name",
			Description: "Name for the new worktree branch",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("branch name is required")
				}
				return nil
			},
		})
		steps = append(steps, components.Step{
			Name:  "Branch name",
			Model: branchInput,
			Summary: func(model any) string {
				if child, ok := model.(components.TextInputModel); ok {
					return child.Value()
				}
				return ""
			},
		})
	}

	branchItems := buildBranchItems(params.Branches, params.DefaultBranch)
	sourceList := components.NewSelectList(components.NewSelectListParams{
		Title:       "Source branch",
		Description: "Branch to base the new worktree on",
		Items:       branchItems,
	})

	steps = append(steps, components.Step{
		Name:  "Source branch",
		Model: sourceList,
		Summary: func(model any) string {
			if child, ok := model.(components.SelectListModel); ok {
				return child.Value()
			}
			return ""
		},
	})

	envItems := buildEnvItems(params.ConfigStrategy)
	envList := components.NewSelectList(components.NewSelectListParams{
		Title:       "Env strategy",
		Description: "How to provision .env files in the new worktree",
		Items:       envItems,
	})

	steps = append(steps, components.Step{
		Name:  "Env strategy",
		Model: envList,
		Summary: func(model any) string {
			child, ok := model.(components.SelectListModel)
			if !ok {
				return ""
			}
			v := child.Value()
			if v == "" {
				return "config default"
			}
			return v
		},
	})

	wiz := components.NewWizard(steps)
	p := tea.NewProgram(wiz)

	finalModel, err := p.Run()
	if err != nil {
		return WizardResult{}, fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return WizardResult{}, domain.ErrUserAborted
	}

	return extractResult(final.Steps(), params.IncludeBranch), nil
}

func extractResult(steps []components.Step, includeBranch bool) WizardResult {
	var result WizardResult
	offset := 0

	if includeBranch {
		if child, ok := steps[0].Model.(components.TextInputModel); ok {
			result.BranchName = child.Value()
		}
		offset = 1
	}

	if child, ok := steps[offset].Model.(components.SelectListModel); ok {
		result.FromBranch = child.Value()
	}
	if child, ok := steps[offset+1].Model.(components.SelectListModel); ok {
		result.EnvFromOverride = child.Value()
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

func buildEnvItems(strategy domain.EnvStrategy) []components.SelectItem {
	return []components.SelectItem{
		{Label: "Use config default (" + string(strategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
}
