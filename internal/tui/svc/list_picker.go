// Package svcpicker renders the interactive pickers for `svc list` and `svc ps`.
package svcpicker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Selection kinds returned by RunListPicker.
const (
	KindProfile = "profile"
	KindService = "service"
)

// List actions emitted by RunListPicker.
const (
	ActionUp    = "up"
	ActionDown  = "down"
	ActionStart = "start"
	ActionStop  = "stop"
	ActionLogs  = "logs"
)

// ListPickerResult describes what the user chose and which action to run.
type ListPickerResult struct {
	Kind   string // KindProfile | KindService
	Name   string
	Action string // one of the ActionX constants
}

// RunListPicker shows a two-step picker: first a mixed profile/service list,
// then a context-dependent action. Returns domain.ErrUserAborted on cancel.
func RunListPicker(cfg domain.ServicesConfig) (ListPickerResult, error) {
	if len(cfg.Profiles) == 0 && len(cfg.Services) == 0 {
		return ListPickerResult{}, domain.ErrUserAborted
	}

	items := buildListItems(cfg)
	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Services & profiles",
		Items: items,
	})
	selected, err := components.RunStandaloneSelect(sl)
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return ListPickerResult{}, domain.ErrUserAborted
		}
		return ListPickerResult{}, err
	}

	kind, name, ok := splitItemValue(selected)
	if !ok {
		return ListPickerResult{}, domain.ErrUserAborted
	}

	action, err := pickListAction(kind, name)
	if err != nil {
		return ListPickerResult{}, err
	}

	return ListPickerResult{Kind: kind, Name: name, Action: action}, nil
}

func buildListItems(cfg domain.ServicesConfig) []components.SelectItem {
	var items []components.SelectItem

	if len(cfg.Profiles) > 0 {
		items = append(items, components.SelectItem{
			Label:    "— Profiles —",
			Disabled: true,
		})
		for _, p := range cfg.Profiles {
			var badges []components.Badge
			if p.Default {
				badges = append(badges, components.Badge{Text: "default", Variant: components.BadgeSuccess})
			}
			badges = append(badges, components.Badge{
				Text:    fmt.Sprintf("%d svc", len(p.Services)),
				Variant: components.BadgeNeutral,
			})
			items = append(items, components.SelectItem{
				Label:  p.Name,
				Value:  KindProfile + ":" + p.Name,
				Badges: badges,
			})
		}
	}

	if len(cfg.Services) > 0 {
		if len(items) > 0 {
			items = append(items, components.SelectItem{Separator: true})
		}
		items = append(items, components.SelectItem{
			Label:    "— Services —",
			Disabled: true,
		})
		for _, s := range cfg.Services {
			items = append(items, components.SelectItem{
				Label: s.Name,
				Value: KindService + ":" + s.Name,
			})
		}
	}

	return items
}

func splitItemValue(v string) (kind, name string, ok bool) {
	idx := strings.IndexByte(v, ':')
	if idx < 0 {
		return "", "", false
	}
	return v[:idx], v[idx+1:], true
}

func pickListAction(kind, name string) (string, error) {
	var items []components.SelectItem
	switch kind {
	case KindProfile:
		items = []components.SelectItem{
			{Label: "Up — start the profile", Value: ActionUp},
			{Label: "Down — stop the profile", Value: ActionDown},
		}
	case KindService:
		items = []components.SelectItem{
			{Label: "Start", Value: ActionStart},
			{Label: "Stop", Value: ActionStop},
			{Label: "Logs", Value: ActionLogs},
		}
	default:
		return "", domain.ErrUserAborted
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: fmt.Sprintf("Action for %s", name),
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
