package run

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var dockerJob = domain.JobConfig{Name: "docker", Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}

func TestStartProfileInlineReportsEachJob(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{Output: map[string][]string{"migrate": {"applied 3 migrations\n"}}}

	err := startProfileInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{migrateJob, apiJob},
	}, &buffers))
	if err != nil {
		t.Fatalf("startProfileInline: %v", err)
	}

	got := buffers.out.String()
	for _, want := range []string{"Running task migrate", "applied 3 migrations", "migrate done", "api started"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
	if !strings.HasPrefix(got, "\n") || !strings.HasSuffix(got, "\n\n") {
		t.Errorf("output is not framed exactly once:\n%q", got)
	}
}

func TestStartProfileInlineSaysAServiceWasAlreadyRunning(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{Refusals: map[string]string{"api": "job api " + domain.JobAlreadyRunningSuffix}}

	if err := startProfileInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{apiJob},
	}, &buffers)); err != nil {
		t.Fatalf("startProfileInline: %v", err)
	}

	if got := buffers.out.String(); !strings.Contains(got, "api already running") {
		t.Errorf("output missing the benign repeat start\n--- output ---\n%s", got)
	}
}

func TestStartProfileInlineReportsAnAbort(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
		Output:   map[string][]string{"migrate": {"relation \"users\" does not exist\n"}},
	}

	err := startProfileInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{dockerJob, migrateJob, apiJob},
	}, &buffers))
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("startProfileInline: %v, want ErrAborted", err)
	}

	report := buffers.err.String()
	for _, want := range []string{"step 2/3", "migrate", "Left running", "docker", "Not started", "api", "run down"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q\n--- report ---\n%s", want, report)
		}
	}
	if strings.Contains(buffers.out.String(), "api started") {
		t.Error("the job after the failed one was started")
	}
}

func TestStartProfileInlineJSONCarriesTheFailedOutput(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
		Output:   map[string][]string{"migrate": {"error: relation ", "\"users\" does not exist\n"}},
	}

	err := startProfileInline(newSurface(surfaceParams{
		Format:  domain.OutputJSON,
		Service: service,
		Start:   []domain.JobConfig{dockerJob, migrateJob, apiJob},
	}, &buffers))
	if err != nil {
		t.Fatalf("startProfileInline: %v, want a parseable document and no error", err)
	}

	var results []domain.JobActionResult
	if err := json.Unmarshal(buffers.out.Bytes(), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, buffers.out.String())
	}

	if len(results) != 2 {
		t.Fatalf("results = %+v, want the started launcher and the failed task", results)
	}
	if results[0].Name != "docker" || results[0].Status != domain.JobActionStarted {
		t.Errorf("results[0] = %+v, want docker started", results[0])
	}
	if results[1].Name != "migrate" || results[1].Status != domain.JobActionError {
		t.Fatalf("results[1] = %+v, want migrate in error", results[1])
	}
	want := "task migrate failed: exit status 1\nerror: relation \"users\" does not exist"
	if results[1].Message != want {
		t.Errorf("message = %q, want %q", results[1].Message, want)
	}
	if buffers.out.Bytes()[0] == '\n' {
		t.Error("the JSON document is framed")
	}
	if got := buffers.err.String(); got != "" {
		t.Errorf("stderr = %q, want a silent machine run", got)
	}
}

func TestJoinJobNames_Empty(t *testing.T) {
	if got := joinJobNames(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestJoinJobNames_One(t *testing.T) {
	got := joinJobNames([]string{"api"})
	if got != "api" {
		t.Errorf("got %q, want %q", got, "api")
	}
}

func TestJoinJobNames_Multiple(t *testing.T) {
	got := joinJobNames([]string{"api", "web", "worker"})
	want := "api, web, worker"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
