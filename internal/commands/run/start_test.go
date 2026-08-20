package run

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var (
	migrateJob = domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"}
	apiJob     = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}
)

type surfaceOutput struct {
	out bytes.Buffer
	err bytes.Buffer
}

func newSurface(params surfaceParams, buffers *surfaceOutput) surfaceParams {
	params.Out = &buffers.out
	params.Err = &buffers.err
	params.WorkDir = "/work/api"
	params.LogDir = "/state/logs/api"
	return params
}

// TestWantsRunViewNeverWithoutATerminal pins the guarantee an agent and a CI
// job depend on: `go test` runs without a terminal, so no combination of flags
// may open the view.
func TestWantsRunViewNeverWithoutATerminal(t *testing.T) {
	formats := []string{domain.OutputText, domain.OutputJSON}
	for _, format := range formats {
		for _, detach := range []bool{false, true} {
			for _, inline := range []bool{false, true} {
				params := wantsRunViewParams{Format: format, Detach: detach, Inline: inline}
				if wantsRunView(params) {
					t.Errorf("wantsRunView(%+v) = true without a terminal", params)
				}
			}
		}
	}
}

func TestStartJobInlineStreamsATask(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{Output: map[string][]string{"migrate": {"applied 3 migrations\n"}}}

	if err := startJobInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{migrateJob},
		Focus:   migrateJob.Name,
	}, &buffers)); err != nil {
		t.Fatalf("startJobInline: %v", err)
	}

	got := buffers.out.String()
	for _, want := range []string{"Running task migrate", "applied 3 migrations", "migrate done"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

func TestStartJobInlineFailsOnTheJobItWasAskedFor(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
		Output:   map[string][]string{"migrate": {"boom\n"}},
	}

	err := startJobInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{migrateJob},
	}, &buffers))
	if err == nil {
		t.Fatal("a failed task must fail the command")
	}
	if err.Error() != "task migrate failed: exit status 1" {
		t.Errorf("error = %q, want the daemon's reason alone — the output already streamed", err)
	}
}

// TestStartJobInlineJSONCarriesTheFailedOutput pins what a machine reader has
// to say why the job did not start: the daemon's reason names the exit code,
// the job's own output says what went wrong.
func TestStartJobInlineJSONCarriesTheFailedOutput(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
		Output:   map[string][]string{"migrate": {"boom\n"}},
	}

	err := startJobInline(newSurface(surfaceParams{
		Format:  domain.OutputJSON,
		Service: service,
		Start:   []domain.JobConfig{migrateJob},
	}, &buffers))
	if err == nil {
		t.Fatal("a failed task must fail the command")
	}
	if err.Error() != "task migrate failed: exit status 1\nboom" {
		t.Errorf("error = %q, want the reason and the output", err)
	}
	if got := buffers.out.String(); got != "" {
		t.Errorf("stdout = %q, want nothing on a run that failed", got)
	}
}

// TestStartJobInlineTreatsARepeatStartAsBenign is the contract attaching by
// default implies: `run start api` on a service that is already up asks for a
// state it is already in, so it opens on the job rather than failing.
func TestStartJobInlineTreatsARepeatStartAsBenign(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{Refusals: map[string]string{"api": "job api " + domain.JobAlreadyRunningSuffix}}

	if err := startJobInline(newSurface(surfaceParams{
		Format:  domain.OutputText,
		Service: service,
		Start:   []domain.JobConfig{apiJob},
	}, &buffers)); err != nil {
		t.Fatalf("startJobInline: %v", err)
	}

	if got := buffers.out.String(); !strings.Contains(got, "api already running") {
		t.Errorf("output = %q, want the repeat start reported as benign", got)
	}
}

func TestStartJobInlineWritesOneJSONResult(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{}

	if err := startJobInline(newSurface(surfaceParams{
		Format:  domain.OutputJSON,
		Service: service,
		Start:   []domain.JobConfig{apiJob},
	}, &buffers)); err != nil {
		t.Fatalf("startJobInline: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal(buffers.out.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, buffers.out.String())
	}
	if result.Name != "api" || result.Status != domain.JobActionStarted {
		t.Errorf("result = %+v, want api started", result)
	}
}

func TestStartInlineSaysNothingInJSON(t *testing.T) {
	var buffers surfaceOutput
	service := &runlogstest.Service{Output: map[string][]string{"migrate": {"applied 3 migrations\n"}}}

	if _, err := startInline(newSurface(surfaceParams{
		Format:  domain.OutputJSON,
		Service: service,
		Start:   []domain.JobConfig{migrateJob},
	}, &buffers)); err != nil {
		t.Fatalf("startInline: %v", err)
	}

	if got := buffers.out.String() + buffers.err.String(); got != "" {
		t.Errorf("a machine-readable run printed %q", got)
	}
}
