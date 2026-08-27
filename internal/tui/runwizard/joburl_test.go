package runwizard

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Editing a job rebuilds it from the wizard's steps, so a field the wizard does
// not carry is a field `run job edit` erases. This is what that looked like for
// a published job before the URL step existed.
func TestExtractJobKeepsThePublishedURL(t *testing.T) {
	steps := make([]components.Step, stepJobURL+1)
	steps[stepJobName] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{Default: "web"})}
	steps[stepJobCmd] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{Default: "pnpm dev"})}
	steps[stepJobKind] = components.Step{Model: components.NewSelectList(components.NewSelectListParams{
		Items: []components.SelectItem{{Label: "service", Value: string(domain.JobKindService)}},
	})}
	steps[stepJobStop] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{})}
	steps[stepJobCwd] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{})}
	steps[stepJobPorts] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{Default: "PORT=3000"})}
	steps[stepJobURL] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{Default: "PORT api.app-1"})}

	job := extractJob(steps)

	if job.URL == nil {
		t.Fatal("editing a published job must not erase its url")
	}
	if job.URL.Port != "PORT" || job.URL.Host != "api.app-1" {
		t.Errorf("url = %+v, want PORT under api.app-1", *job.URL)
	}
}

func TestExtractJobLeavesAnUnpublishedJobUnpublished(t *testing.T) {
	steps := make([]components.Step, stepJobURL+1)
	for i := range steps {
		steps[i] = components.Step{Model: components.NewTextInput(components.NewTextInputParams{})}
	}
	steps[stepJobKind] = components.Step{Model: components.NewSelectList(components.NewSelectListParams{
		Items: []components.SelectItem{{Label: "service", Value: string(domain.JobKindService)}},
	})}

	if job := extractJob(steps); job.URL != nil {
		t.Errorf("url = %+v, want none", job.URL)
	}
}
