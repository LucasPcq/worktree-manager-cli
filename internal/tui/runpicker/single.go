package runpicker

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// PickJobParams holds the inputs for PickJob.
type PickJobParams struct {
	Config domain.RunConfig
	Title  string
}

// PickJob prompts the user to pick one job from the run config. Returns the
// job name or domain.ErrUserAborted on cancel. Errors when no jobs are declared.
func PickJob(params PickJobParams) (string, error) {
	if len(params.Config.Jobs) == 0 {
		return "", fmt.Errorf("no jobs declared in run.toml")
	}
	return runStandalonePick(params.Title, buildJobSelectItems(params.Config))
}

// PickProfileParams holds the inputs for PickProfile.
type PickProfileParams struct {
	Config domain.RunConfig
	Title  string
}

// PickProfile prompts the user to pick one profile from the run config.
// Returns the profile name or domain.ErrUserAborted on cancel. Errors when
// no profiles are declared.
func PickProfile(params PickProfileParams) (string, error) {
	if len(params.Config.Profiles) == 0 {
		return "", fmt.Errorf("no profiles declared in run.toml")
	}
	return runStandalonePick(params.Title, buildProfileSelectItems(params.Config))
}

func buildJobSelectItems(cfg domain.RunConfig) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(cfg.Jobs))
	for _, j := range cfg.Jobs {
		items = append(items, components.SelectItem{
			Label:  fmt.Sprintf("%s — %s", j.Name, j.Cmd),
			Value:  j.Name,
			Badges: []components.Badge{{Text: string(j.Kind), Variant: components.BadgeNeutral}},
		})
	}
	return items
}

func buildProfileSelectItems(cfg domain.RunConfig) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		var badges []components.Badge
		if p.Default {
			badges = append(badges, components.Badge{Text: "default", Variant: components.BadgeSuccess})
		}
		items = append(items, components.SelectItem{
			Label:  fmt.Sprintf("%s — %s", p.Name, strings.Join(p.Jobs, ", ")),
			Value:  p.Name,
			Badges: badges,
		})
	}
	return items
}

func runStandalonePick(title string, items []components.SelectItem) (string, error) {
	list := components.NewSelectList(components.NewSelectListParams{
		Title: title,
		Items: items,
	})
	picked, err := components.RunStandaloneSelect(list)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return "", domain.ErrUserAborted
		}
		return "", err
	}
	return picked, nil
}

// CRUD actions emitted by PickJobThenAction / PickProfileThenAction.
const (
	ActionEdit = "edit"
	ActionRm   = "rm"
)

// JobActionPick is the result of PickJobThenAction.
type JobActionPick struct {
	Name   string
	Action string
}

// ProfileActionPick is the result of PickProfileThenAction.
type ProfileActionPick struct {
	Name   string
	Action string
}

// PickJobThenAction shows a 2-step wizard: job picker, then Edit/Remove.
// Esc on step 2 navigates back to the job picker; Esc on step 1 aborts.
func PickJobThenAction(cfg domain.RunConfig) (JobActionPick, error) {
	if len(cfg.Jobs) == 0 {
		return JobActionPick{}, fmt.Errorf("no jobs declared in run.toml")
	}

	jobList := components.NewSelectList(components.NewSelectListParams{
		Title: "Choose a job",
		Items: buildJobSelectItems(cfg),
	})

	steps := []components.Step{
		{Name: "Job", Model: jobList, Summary: components.SelectSummary},
		{Name: "Action", Model: newActionList(), Summary: components.SelectSummary},
	}

	final, err := runActionWizard(steps)
	if err != nil {
		return JobActionPick{}, err
	}

	return JobActionPick{
		Name:   stepValue(final, 0),
		Action: stepValue(final, 1),
	}, nil
}

// PickProfileThenAction shows a 2-step wizard: profile picker, then
// Edit/Remove. Esc on step 2 navigates back; Esc on step 1 aborts.
func PickProfileThenAction(cfg domain.RunConfig) (ProfileActionPick, error) {
	if len(cfg.Profiles) == 0 {
		return ProfileActionPick{}, fmt.Errorf("no profiles declared in run.toml")
	}

	profileList := components.NewSelectList(components.NewSelectListParams{
		Title: "Choose a profile",
		Items: buildProfileSelectItems(cfg),
	})

	steps := []components.Step{
		{Name: "Profile", Model: profileList, Summary: components.SelectSummary},
		{Name: "Action", Model: newActionList(), Summary: components.SelectSummary},
	}

	final, err := runActionWizard(steps)
	if err != nil {
		return ProfileActionPick{}, err
	}

	return ProfileActionPick{
		Name:   stepValue(final, 0),
		Action: stepValue(final, 1),
	}, nil
}

func newActionList() components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title: "Action",
		Items: []components.SelectItem{
			{Label: "Edit", Value: ActionEdit},
			{Label: "Remove", Value: ActionRm},
		},
	})
}

func runActionWizard(steps []components.Step) (components.WizardModel, error) {
	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return components.WizardModel{}, fmt.Errorf("picker: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return components.WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

func stepValue(final components.WizardModel, idx int) string {
	if c, ok := final.Steps()[idx].Model.(components.SelectListModel); ok {
		return c.Value()
	}
	return ""
}
