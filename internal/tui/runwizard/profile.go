package runwizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	stepProfileName = iota
	stepProfileJobs
	stepProfileDefault
)

// ProfileWizardParams configures the profile wizard for both add and edit flows.
type ProfileWizardParams struct {
	Existing    domain.RunConfig
	Initial     domain.ProfileConfig
	ExcludeName string
}

// RunProfileWizard prompts the user for a profile's fields and returns the
// resulting ProfileConfig. Returns domain.ErrUserAborted on Esc at the first
// step. Errors when no jobs are declared (no items to multiselect from).
func RunProfileWizard(params ProfileWizardParams) (domain.ProfileConfig, error) {
	if len(params.Existing.Jobs) == 0 {
		return domain.ProfileConfig{}, fmt.Errorf("cannot define a profile: no jobs declared yet — add a job first")
	}

	nameInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Profile name",
		Description: "Unique identifier for this profile",
		Default:     params.Initial.Name,
		Validate: func(s string) error {
			name := strings.TrimSpace(s)
			if name == "" {
				return fmt.Errorf("profile name is required")
			}
			if name == params.ExcludeName {
				return nil
			}
			if _, exists := rules.FindProfile(params.Existing, name); exists {
				return fmt.Errorf("profile %q already exists", name)
			}
			return nil
		},
	})

	jobsSelect := components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Jobs",
		Description: "Select jobs to include — space toggles, enter confirms",
		Items:       buildJobItems(params.Existing, params.Initial.Jobs),
		Validate: func(values []string) error {
			if len(values) == 0 {
				return fmt.Errorf("select at least one job")
			}
			return nil
		},
	})

	var defaultWarning string
	if existingDefault := rules.FindExistingDefaultProfile(params.Existing, params.ExcludeName); existingDefault != "" {
		defaultWarning = fmt.Sprintf("Profile %q is currently default — confirming yes will replace it.", existingDefault)
	}
	defaultConfirm := components.NewConfirm(components.NewConfirmParams{
		Title:       "Default profile?",
		Description: "Mark this profile as default. Only one profile can be the default.",
		Warning:     defaultWarning,
		DefaultYes:  params.Initial.Default,
	})

	steps := []components.Step{
		{Name: "Name", Model: nameInput, Summary: components.TextSummary},
		{Name: "Jobs", Model: jobsSelect, Summary: components.MultiSelectSummary("(none)")},
		{Name: "Default", Model: defaultConfirm, Summary: components.ConfirmSummary("yes", "no")},
	}

	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.ProfileConfig{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.ProfileConfig{}, domain.ErrUserAborted
	}

	return extractProfile(final.Steps()), nil
}

// buildJobItems lists existing jobs in the multiselect, putting jobs already
// in the profile first (in their declared order) and pre-selecting them.
func buildJobItems(cfg domain.RunConfig, selected []string) []components.MultiSelectItem {
	items := make([]components.MultiSelectItem, 0, len(cfg.Jobs))
	seen := make(map[string]bool, len(selected))

	for _, name := range selected {
		j, ok := rules.FindJob(cfg, name)
		if !ok {
			continue
		}
		items = append(items, components.MultiSelectItem{
			Label:    fmt.Sprintf("%s (%s)", j.Name, j.Kind),
			Value:    j.Name,
			Selected: true,
		})
		seen[j.Name] = true
	}

	for _, j := range cfg.Jobs {
		if seen[j.Name] {
			continue
		}
		items = append(items, components.MultiSelectItem{
			Label: fmt.Sprintf("%s (%s)", j.Name, j.Kind),
			Value: j.Name,
		})
	}

	return items
}

func extractProfile(steps []components.Step) domain.ProfileConfig {
	var p domain.ProfileConfig
	if c, ok := steps[stepProfileName].Model.(components.TextInputModel); ok {
		p.Name = c.Value()
	}
	if c, ok := steps[stepProfileJobs].Model.(components.MultiSelectModel); ok {
		p.Jobs = c.Values()
	}
	if c, ok := steps[stepProfileDefault].Model.(components.ConfirmModel); ok {
		p.Default = c.Confirmed()
	}
	return p
}
