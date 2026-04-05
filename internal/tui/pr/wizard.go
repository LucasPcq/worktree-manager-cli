// Package pr builds interactive huh forms for the wtm pr commands.
package pr

import (
	"errors"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
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
	title := params.DefaultTitle
	draft := params.AutoDraft

	branchOptions := buildBranchOptions(params.Branches, params.DefaultBase)

	var selectedBase string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("PR title").
				Value(&title),
			huh.NewSelect[string]().
				Title("Base branch").
				Description("Target branch for the PR").
				Options(branchOptions...).
				Filtering(true).
				Value(&selectedBase),
			huh.NewSelect[bool]().
				Title("Draft").
				Options(
					huh.NewOption("No — ready for review", false),
					huh.NewOption("Yes — work in progress", true),
				).
				Value(&draft),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return CreateWizardResult{}, domain.ErrUserAborted
		}
		return CreateWizardResult{}, err
	}

	return CreateWizardResult{
		Title: title,
		Base:  selectedBase,
		Draft: draft,
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
