package runpicker

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunJobPicker asks which declared job to act on. `run start` and `run stop`
// reach it only in a fully interactive run; every other surface names the job
// with --job or errors.
func RunJobPicker(jobs []domain.JobConfig) (domain.JobConfig, error) {
	items := make([]components.SelectItem, 0, len(jobs))
	for _, job := range jobs {
		var badges []components.Badge
		if job.Kind != "" {
			badges = append(badges, components.Badge{Text: string(job.Kind), Variant: components.BadgeNeutral})
		}
		items = append(items, components.SelectItem{Label: job.Name, Value: job.Name, Badges: badges})
	}

	name, err := components.RunStandaloneSelect(components.NewSelectList(components.NewSelectListParams{
		Title: domain.RunJobPickerTitle,
		Items: items,
	}))
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.JobConfig{}, domain.ErrUserAborted
		}
		return domain.JobConfig{}, err
	}

	for _, job := range jobs {
		if job.Name == name {
			return job, nil
		}
	}
	return domain.JobConfig{}, domain.ErrUserAborted
}
