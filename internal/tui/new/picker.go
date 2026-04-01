// Package new builds interactive huh forms for the wtm new command.
package new

import (
	"errors"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
)

// WizardParams holds inputs for the interactive new worktree wizard.
type WizardParams struct {
	Branches       []string
	DefaultBranch  string
	ConfigStrategy domain.EnvStrategy
}

// WizardResult holds the answers from the interactive wizard.
type WizardResult struct {
	FromBranch      string
	EnvFromOverride string
}

// RunWizard displays the interactive wizard for wtm new.
// Asks for source branch and env strategy override. Returns ErrUserAborted on Ctrl+C.
func RunWizard(params WizardParams) (WizardResult, error) {
	branchOptions := buildBranchOptions(params.Branches, params.DefaultBranch)

	var selectedBranch string
	var envStrategy string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Source branch").
				Description("Branch to base the new worktree on").
				Options(branchOptions...).
				Filtering(true).
				Value(&selectedBranch),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Env strategy").
				Description("How to provision .env files in the new worktree").
				Options(
					huh.NewOption("Use config default ("+string(params.ConfigStrategy)+")", ""),
					huh.NewOption("example — copy .env.example → .env", string(domain.EnvStrategyExample)),
					huh.NewOption("main — copy .env from main worktree", string(domain.EnvStrategyMain)),
					huh.NewOption("parent — copy .env from source worktree", string(domain.EnvStrategyParent)),
				).
				Value(&envStrategy),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return WizardResult{}, domain.ErrUserAborted
		}
		return WizardResult{}, err
	}

	return WizardResult{
		FromBranch:      selectedBranch,
		EnvFromOverride: envStrategy,
	}, nil
}

func buildBranchOptions(branches []string, defaultBranch string) []huh.Option[string] {
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

	options := make([]huh.Option[string], 0, len(branches))

	if hasDefault {
		options = append(options, huh.NewOption(defaultBranch+" (default)", defaultBranch))
	}

	for _, b := range rest {
		options = append(options, huh.NewOption(b, b))
	}

	return options
}
