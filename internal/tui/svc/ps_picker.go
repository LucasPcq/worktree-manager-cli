package svcpicker

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
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
// Distinct from any real service name.
const sentinelStopAll = "__wtm_stop_all__"

// PsPickerResult describes the selected running service and chosen action.
type PsPickerResult struct {
	Name   string
	Action string
}

// RunPsPicker shows the running-services picker with contextual actions.
// Returns domain.ErrUserAborted on cancel.
func RunPsPicker(services []process.ServiceInfo) (PsPickerResult, error) {
	if len(services) == 0 {
		return PsPickerResult{}, domain.ErrUserAborted
	}

	items := make([]components.SelectItem, 0, len(services)+2)
	for _, s := range services {
		badges := []components.Badge{statusBadge(s.Status)}
		if s.WorkDir != "" {
			badges = append(badges, components.Badge{
				Text:    filepath.Base(s.WorkDir),
				Variant: components.BadgeNeutral,
			})
		}
		items = append(items, components.SelectItem{
			Label:  s.Name,
			Value:  s.Name,
			Badges: badges,
		})
	}
	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{
			Label:  "Stop all running services",
			Value:  sentinelStopAll,
			Danger: true,
		},
	)

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Running services",
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

	selected := findService(services, selectedName)
	action, err := pickPsAction(selected)
	if err != nil {
		return PsPickerResult{}, err
	}
	return PsPickerResult{Name: selected.Name, Action: action}, nil
}

func findService(services []process.ServiceInfo, name string) process.ServiceInfo {
	for _, s := range services {
		if s.Name == name {
			return s
		}
	}
	return process.ServiceInfo{Name: name}
}

func pickPsAction(svc process.ServiceInfo) (string, error) {
	var items []components.SelectItem
	if svc.Status == domain.ServiceStatusRunning {
		items = []components.SelectItem{
			{Label: "Stop", Value: ActionPsStop},
			{Label: "Logs (attach)", Value: ActionPsLogs},
		}
	} else {
		items = []components.SelectItem{
			{Label: "Restart", Value: ActionPsRestart},
			{Label: "Remove (stop)", Value: ActionPsStop},
		}
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: fmt.Sprintf("Action for %s", svc.Name),
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

func statusBadge(status domain.ServiceStatus) components.Badge {
	switch status {
	case domain.ServiceStatusRunning:
		return components.Badge{Text: string(status), Variant: components.BadgeSuccess}
	case domain.ServiceStatusCrashed:
		return components.Badge{Text: string(status), Variant: components.BadgeWarning}
	default:
		return components.Badge{Text: string(status), Variant: components.BadgeNeutral}
	}
}
