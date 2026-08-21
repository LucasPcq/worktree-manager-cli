package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// The user opening a second worktree wants one thing from the start line: the
// port their copy is on.
func TestRunPrinterReportsBoundPorts(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{
		Phase: runlogs.PhaseStarted,
		Job:   "web",
		Ports: map[string]int{"PORT": 3010},
	})

	if !strings.Contains(out.String(), "web started") || !strings.Contains(out.String(), "PORT=3010") {
		t.Errorf("got %q", out.String())
	}
}

func TestRunPrinterLeavesAPortlessJobAlone(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web"})

	if strings.Contains(out.String(), "·") {
		t.Errorf("nothing to say should say nothing, got %q", out.String())
	}
}
