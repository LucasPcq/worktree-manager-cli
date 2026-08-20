package run

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func TestReportProfileAbort(t *testing.T) {
	var report bytes.Buffer

	reportProfileAbort(&report, runlogs.Outcome{
		Started:    []string{"docker"},
		Failed:     "migrate",
		FailedStep: 2,
		Steps:      3,
		NotStarted: []string{"dev"},
	})

	got := report.String()
	for _, want := range []string{"step 2/3", "migrate", "Left running", "docker", "Not started", "dev", "run down"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q\n--- report ---\n%s", want, got)
		}
	}
}

func TestReportProfileAbortLastStepNoNotStarted(t *testing.T) {
	var report bytes.Buffer

	reportProfileAbort(&report, runlogs.Outcome{Failed: "migrate", FailedStep: 2, Steps: 2})

	got := report.String()
	if strings.Contains(got, "Not started") {
		t.Errorf("expected no 'Not started' line when the last job fails, got:\n%s", got)
	}
	if strings.Contains(got, "Left running") {
		t.Errorf("expected no 'Left running' line when nothing was left running, got:\n%s", got)
	}
}
