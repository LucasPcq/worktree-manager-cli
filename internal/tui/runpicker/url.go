package runpicker

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunURLPicker asks which published job to open. It is only ever reached from a
// fully interactive run: every other surface names the job or errors.
func RunURLPicker(entries []domain.JobURLEntry) (domain.JobURLEntry, error) {
	items := make([]components.SelectItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, components.SelectItem{
			Label:  entry.Job,
			Value:  entry.Job,
			Badges: []components.Badge{{Text: entry.URL, Variant: components.BadgeNeutral}},
		})
	}

	name, err := components.RunStandaloneSelect(components.NewSelectList(components.NewSelectListParams{
		Title: domain.RunURLPickerTitle,
		Items: items,
	}))
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.JobURLEntry{}, domain.ErrUserAborted
		}
		return domain.JobURLEntry{}, err
	}

	for _, entry := range entries {
		if entry.Job == name {
			return entry, nil
		}
	}
	return domain.JobURLEntry{}, domain.ErrUserAborted
}
