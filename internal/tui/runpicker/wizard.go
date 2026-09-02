package runpicker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// WorktreeStep asks which worktree the command acts on.
type WorktreeStep struct {
	Worktrees []domain.GitWorktree
	// Current is the path the cursor opens on — the worktree the command was
	// launched from, so Enter reproduces acting where you stand.
	Current string
	// Running counts the jobs up in each worktree, keyed by the path git spells.
	Running map[string]int
}

// SecondStep asks what to act on inside that worktree. Exactly one of its
// three lists is set; which one decides how an entry is labelled.
type SecondStep struct {
	Jobs     []domain.JobConfig
	Profiles []domain.ProfileConfig
	URLs     []domain.JobURLEntry
	Start    string
	// ResolveURLs, when set, recomputes the entries from the worktree chosen on
	// the step before. A job's address depends on that worktree's ordinal, so a
	// list built once would show the ports of wherever the command was launched.
	ResolveURLs func(worktreePath string) []domain.JobURLEntry
}

type TargetWizardParams struct {
	// Worktree and Second are each asked only when set. A question the caller
	// already has an answer for is not a step, so it neither costs a keystroke
	// nor appears in the breadcrumb.
	Worktree *WorktreeStep
	Second   *SecondStep
}

type TargetWizardResult struct {
	WorktreePath string
	Second       string
}

// RunTargetWizard asks a run command's outstanding questions as one form rather
// than as a sequence of standalone pickers. That is the whole point of it being
// a wizard: separate programs can only abort, so cancelling the second question
// threw away the answer to the first instead of stepping back to it.
func RunTargetWizard(params TargetWizardParams) (TargetWizardResult, error) {
	steps, indexes := buildTargetSteps(params)
	if len(steps) == 0 {
		return TargetWizardResult{}, nil
	}

	finalModel, err := tea.NewProgram(components.NewWizard(steps)).Run()
	if err != nil {
		return TargetWizardResult{}, fmt.Errorf("run wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return TargetWizardResult{}, domain.ErrUserAborted
	}

	return TargetWizardResult{
		WorktreePath: targetStepValue(final, indexes.worktree),
		Second:       targetStepValue(final, indexes.second),
	}, nil
}

// stepIndexes records where each question landed, since a question the caller
// already answered is not added at all.
type stepIndexes struct {
	worktree int
	second   int
}

func buildTargetSteps(params TargetWizardParams) ([]components.Step, stepIndexes) {
	indexes := stepIndexes{worktree: -1, second: -1}
	var steps []components.Step

	if params.Worktree != nil {
		indexes.worktree = len(steps)
		steps = append(steps, components.Step{
			Name: domain.RunWorktreeStepName,
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       domain.RunWorktreePickerTitle,
				Description: domain.RunWorktreePickerDesc,
				Items:       worktreeItems(*params.Worktree),
				Start:       params.Worktree.Current,
			}),
			Summary: selectSummary,
		})
	}

	if params.Second != nil {
		second := *params.Second
		name, title, desc, items := secondContent(second)
		indexes.second = len(steps)
		step := components.Step{
			Name: name,
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       title,
				Description: desc,
				Items:       items,
				Start:       second.Start,
			}),
			Summary: selectSummary,
		}
		if second.ResolveURLs != nil && indexes.worktree >= 0 {
			worktreeIndex := indexes.worktree
			step.Build = func(prev []components.Step) any {
				return components.NewSelectList(components.NewSelectListParams{
					Title: title,
					Items: urlItems(second.ResolveURLs(stepAnswer(prev, worktreeIndex))),
				})
			}
		}
		steps = append(steps, step)
	}

	return steps, indexes
}

func worktreeItems(step WorktreeStep) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(step.Worktrees))
	for _, wt := range step.Worktrees {
		var badges []components.Badge
		if count := step.Running[wt.Path]; count > 0 {
			badges = append(badges, components.Badge{
				Text:    fmt.Sprintf(domain.RunWorktreeJobsFmt, count),
				Variant: components.BadgeSuccess,
			})
		}
		if wt.Path == step.Current {
			badges = append(badges, components.Badge{Text: domain.RunWorktreeCurrent, Variant: components.BadgeNeutral})
		}
		items = append(items, components.SelectItem{Label: wt.Branch, Value: wt.Path, Badges: badges})
	}
	return items
}

func secondContent(step SecondStep) (name, title, desc string, items []components.SelectItem) {
	switch {
	case len(step.Profiles) > 0:
		return domain.RunProfileStepName, domain.RunProfilePickerTitle, domain.RunProfilePickerDesc, profileItems(step.Profiles)
	case len(step.URLs) > 0:
		return domain.RunJobStepName, domain.RunURLPickerTitle, "", urlItems(step.URLs)
	default:
		return domain.RunJobStepName, domain.RunJobPickerTitle, "", jobItems(step.Jobs)
	}
}

func profileItems(profiles []domain.ProfileConfig) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(profiles))
	for _, p := range profiles {
		label := p.Name
		if len(p.Jobs) > 0 {
			label += fmt.Sprintf(" (%s)", strings.Join(p.Jobs, domain.RunURLListSep))
		}
		items = append(items, components.SelectItem{Label: label, Value: p.Name})
	}
	return items
}

func jobItems(jobs []domain.JobConfig) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(jobs))
	for _, job := range jobs {
		var badges []components.Badge
		if job.Kind != "" {
			badges = append(badges, components.Badge{Text: string(job.Kind), Variant: components.BadgeNeutral})
		}
		items = append(items, components.SelectItem{Label: job.Name, Value: job.Name, Badges: badges})
	}
	return items
}

func urlItems(entries []domain.JobURLEntry) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, components.SelectItem{
			Label:  entry.Job,
			Value:  entry.Job,
			Badges: []components.Badge{{Text: entry.URL, Variant: components.BadgeNeutral}},
		})
	}
	return items
}

// stepAnswer reads a completed step's value, for a later step that is built from
// it.
func stepAnswer(steps []components.Step, index int) string {
	if index < 0 || index >= len(steps) {
		return ""
	}
	sl, ok := steps[index].Model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func selectSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func targetStepValue(wizard components.WizardModel, index int) string {
	if index < 0 {
		return ""
	}
	sl, ok := wizard.Steps()[index].Model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}
