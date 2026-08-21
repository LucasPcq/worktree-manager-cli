// Package runwizard hosts the create/edit wizards for run.toml entries —
// jobs and profiles. Mirrors the layering of tui/inittui (one package
// regrouping multiple feature wizards).
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
	stepJobName = iota
	stepJobCmd
	stepJobKind
	stepJobStop
	stepJobCwd
	stepJobPorts
)

// JobWizardParams configures the job wizard for both add and edit flows.
// In edit mode, Initial holds the current values and ExcludeName is the
// current name (allowed to remain unchanged); in add mode both are zero.
type JobWizardParams struct {
	Existing    domain.RunConfig
	Initial     domain.JobConfig
	ExcludeName string
}

// RunJobWizard prompts the user for a job's fields and returns the resulting
// JobConfig. Returns domain.ErrUserAborted on Esc at the first step.
func RunJobWizard(params JobWizardParams) (domain.JobConfig, error) {
	nameInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Job name",
		Description: "Unique identifier for this job",
		Default:     params.Initial.Name,
		Validate: func(s string) error {
			name := strings.TrimSpace(s)
			if name == "" {
				return fmt.Errorf("job name is required")
			}
			if name == params.ExcludeName {
				return nil
			}
			if _, exists := rules.FindJob(params.Existing, name); exists {
				return fmt.Errorf("job %q already exists", name)
			}
			return nil
		},
	})

	cmdInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Command",
		Description: "Shell command to execute",
		Default:     params.Initial.Cmd,
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("command is required")
			}
			return nil
		},
	})

	kindList := components.NewSelectList(components.NewSelectListParams{
		Title:       "Job kind",
		Description: "service = long-running, task = one-shot",
		Items:       buildKindItems(params.Initial.Kind),
	})

	stopInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Stop command",
		Description: "Optional. Services only — leave blank for default SIGTERM, or set for detached pattern (e.g. docker compose down).",
		Default:     params.Initial.Stop,
	})

	cwdInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Working directory",
		Description: "Optional. Relative to project root.",
		Default:     params.Initial.Cwd,
	})

	portsInput := components.NewTextInput(components.NewTextInputParams{
		Title:       "Base ports",
		Description: "Optional. NAME=PORT, space-separated (e.g. PORT=3000 DB_PORT=5432). Each worktree gets the base plus its own offset.",
		Default:     strings.Join(rules.PortEntries(params.Initial.Ports), " "),
		Validate: func(s string) error {
			_, err := rules.ParsePorts(strings.Fields(s))
			return err
		},
	})

	steps := []components.Step{
		{Name: "Name", Model: nameInput, Summary: components.TextSummary},
		{Name: "Command", Model: cmdInput, Summary: components.TextSummary},
		{Name: "Kind", Model: kindList, Summary: components.SelectSummary},
		{Name: "Stop", Model: stopInput, Summary: components.OptionalTextSummary("(none)")},
		{Name: "Cwd", Model: cwdInput, Summary: components.OptionalTextSummary("(project root)")},
		{Name: "Ports", Model: portsInput, Summary: components.OptionalTextSummary("(none)")},
	}

	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.JobConfig{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.JobConfig{}, domain.ErrUserAborted
	}

	return extractJob(final.Steps()), nil
}

// buildKindItems puts the current kind first so the cursor lands on it.
func buildKindItems(current domain.JobKind) []components.SelectItem {
	service := components.SelectItem{Label: "service — long-running (e.g. dev server)", Value: string(domain.JobKindService)}
	task := components.SelectItem{Label: "task — one-shot (e.g. build, migrate)", Value: string(domain.JobKindTask)}
	if current == domain.JobKindTask {
		return []components.SelectItem{task, service}
	}
	return []components.SelectItem{service, task}
}

func extractJob(steps []components.Step) domain.JobConfig {
	var job domain.JobConfig
	if c, ok := steps[stepJobName].Model.(components.TextInputModel); ok {
		job.Name = c.Value()
	}
	if c, ok := steps[stepJobCmd].Model.(components.TextInputModel); ok {
		job.Cmd = c.Value()
	}
	if c, ok := steps[stepJobKind].Model.(components.SelectListModel); ok {
		job.Kind = domain.JobKind(c.Value())
	}
	if c, ok := steps[stepJobStop].Model.(components.TextInputModel); ok {
		job.Stop = c.Value()
	}
	if c, ok := steps[stepJobCwd].Model.(components.TextInputModel); ok {
		job.Cwd = c.Value()
	}
	if c, ok := steps[stepJobPorts].Model.(components.TextInputModel); ok {
		// The step validated the line, so a parse error here cannot happen; a
		// job with no ports is the same job it was before.
		job.Ports, _ = rules.ParsePorts(strings.Fields(c.Value()))
	}
	return job
}
