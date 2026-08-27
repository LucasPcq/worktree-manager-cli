package runview

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Result is what the view leaves behind once the alternate screen is given
// back. Recap is a raw body: the command that opened the view is the one that
// frames it.
type Result struct {
	Recap   string
	Outcome runlogs.Outcome
}

func (m Model) result() Result {
	return Result{Recap: m.recap(), Outcome: m.sequence.outcome}
}

// recap is the last thing said about a run, on the terminal the view was
// covering: what it left running, and the two commands that act on it. A view
// that started nothing — `wtm run logs` — has nothing to conclude.
func (m Model) recap() string {
	if !m.started {
		return ""
	}

	outcome := m.sequence.outcome
	var lines []string
	if m.profile != "" {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapProfileFmt, m.profile))
	}
	if len(outcome.Started) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapRunningFmt, joinJobs(outcome.Started)))
	}
	if len(outcome.Completed) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapCompletedFmt, joinJobs(outcome.Completed)))
	}
	if outcome.Failed != "" {
		lines = append(lines, styles.DangerText.Render(fmt.Sprintf(domain.RunViewRecapFailedFmt, outcome.Failed)))
	}
	if len(outcome.NotStarted) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapNotStartedFmt, joinJobs(outcome.NotStarted)))
	}
	if len(outcome.Started) == 0 {
		lines = append(lines, domain.RunViewRecapNoneRunning)
	}

	hints := []string{styles.Muted.Render(domain.RunViewRecapLogsHint)}
	if len(outcome.Started) > 0 {
		hints = append(hints, styles.Muted.Render(domain.RunViewRecapDownHint))
	}
	body := strings.Join(append(lines, append([]string{""}, hints...)...), "\n")

	return styles.RenderRecap(styles.IntroParams{
		Width: m.width,
		Title: domain.RunViewRecapTitle,
		Body:  body,
	})
}
