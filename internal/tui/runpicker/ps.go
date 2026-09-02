package runpicker

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Ps actions emitted by RunPsPicker.
const (
	ActionPsStop    = "stop"
	ActionPsLogs    = "logs"
	ActionPsRestart = "restart"
	ActionPsStopAll = "stop-all"
)

// sentinelStopAll is the item value used to signal the global stop-all entry.
// Distinct from any real job name.
const sentinelStopAll = "__wtm_stop_all__"

// PsPickerResult describes the selected running job and chosen action.
type PsPickerResult struct {
	Name   string
	Action string
}

// RunPsPicker shows the running-jobs picker with contextual actions.
// Returns domain.ErrUserAborted on cancel.
func RunPsPicker(jobs []domain.JobInfo) (PsPickerResult, error) {
	if len(jobs) == 0 {
		return PsPickerResult{}, domain.ErrUserAborted
	}

	items := make([]components.SelectItem, 0, len(jobs)+2)
	for _, j := range jobs {
		badges := []components.Badge{statusBadge(j.Status)}
		if j.Kind != "" {
			badges = append(badges, components.Badge{Text: string(j.Kind), Variant: components.BadgeNeutral})
		}
		if j.WorkDir != "" {
			badges = append(badges, components.Badge{
				Text:    filepath.Base(j.WorkDir),
				Variant: components.BadgeNeutral,
			})
		}
		items = append(items, components.SelectItem{
			Label:  j.Name,
			Value:  j.Name,
			Badges: badges,
		})
	}
	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{
			Label:  "Stop all running jobs",
			Value:  sentinelStopAll,
			Danger: true,
		},
	)

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Running jobs",
		Items: items,
	})
	selectedName, err := components.RunStandaloneSelect(sl)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return PsPickerResult{}, domain.ErrUserAborted
		}
		return PsPickerResult{}, err
	}

	if selectedName == sentinelStopAll {
		return PsPickerResult{Action: ActionPsStopAll}, nil
	}

	selected := findJob(jobs, selectedName)
	action, err := pickPsAction(selected)
	if err != nil {
		return PsPickerResult{}, err
	}
	return PsPickerResult{Name: selected.Name, Action: action}, nil
}

func findJob(jobs []domain.JobInfo, name string) domain.JobInfo {
	for _, j := range jobs {
		if j.Name == name {
			return j
		}
	}
	return domain.JobInfo{Name: name}
}

func pickPsAction(job domain.JobInfo) (string, error) {
	var items []components.SelectItem
	switch {
	case job.Status == domain.JobStatusRunning:
		items = []components.SelectItem{
			{Label: "Stop", Value: ActionPsStop},
			{Label: "Logs (attach)", Value: ActionPsLogs},
		}
	case job.Status == domain.JobStatusDetached:
		// No attach: the launcher's stream ended with it. Restarting is offered
		// because a detached launcher is idempotent, which is what makes it
		// detachable in the first place.
		items = []components.SelectItem{
			{Label: "Stop", Value: ActionPsStop},
			{Label: "Restart", Value: ActionPsRestart},
		}
	default:
		items = []components.SelectItem{
			{Label: "Restart", Value: ActionPsRestart},
			{Label: "Remove (stop)", Value: ActionPsStop},
		}
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: fmt.Sprintf("Action for %s", job.Name),
		Items: items,
	})
	action, err := components.RunStandaloneSelect(sl)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return "", domain.ErrUserAborted
		}
		return "", err
	}
	return action, nil
}

func statusBadge(status domain.JobStatus) components.Badge {
	switch status {
	case domain.JobStatusRunning, domain.JobStatusDetached:
		return components.Badge{Text: string(status), Variant: components.BadgeSuccess}
	case domain.JobStatusCrashed:
		return components.Badge{Text: string(status), Variant: components.BadgeWarning}
	default:
		return components.Badge{Text: string(status), Variant: components.BadgeNeutral}
	}
}
