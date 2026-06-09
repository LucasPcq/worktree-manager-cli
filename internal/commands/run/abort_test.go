package run

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestReportProfileAbort(t *testing.T) {
	jobs := []domain.JobConfig{
		{Name: "docker", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
		{Name: "dev", Kind: domain.JobKindService},
	}
	started := []domain.JobConfig{{Name: "docker", Kind: domain.JobKindService}}

	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	reportProfileAbort(cmd, jobs, 1, started)
	got := errBuf.String()

	for _, want := range []string{"step 2/3", "migrate", "Left running", "docker", "Not started", "dev", "run down"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q\n--- report ---\n%s", want, got)
		}
	}
}

func TestReportProfileAbort_LastStepNoNotStarted(t *testing.T) {
	jobs := []domain.JobConfig{
		{Name: "build", Kind: domain.JobKindTask},
		{Name: "migrate", Kind: domain.JobKindTask},
	}

	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	// Failure on the final job: nothing left to start, no services started.
	reportProfileAbort(cmd, jobs, 1, nil)
	got := errBuf.String()

	if strings.Contains(got, "Not started") {
		t.Errorf("expected no 'Not started' line when the last job fails, got:\n%s", got)
	}
	if strings.Contains(got, "Left running") {
		t.Errorf("expected no 'Left running' line when no service started, got:\n%s", got)
	}
}
