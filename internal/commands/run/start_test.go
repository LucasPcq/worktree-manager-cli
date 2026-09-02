package run

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

var (
	apiJob     = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}
	migrateJob = domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"}
)

func setupStartProject(t *testing.T, daemon *fakeDaemon) *fakeDaemon {
	t.Helper()

	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{apiJob, migrateJob},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"migrate", "api"}, Default: true},
		},
	})
	return startFakeDaemon(t, daemon)
}

func TestRunStartAttachesAServiceByDefault(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api"); err != nil {
		t.Fatalf("run start api: %v", err)
	}

	call := view.only(t)
	if call.Job != "api" {
		t.Errorf("the view opened on %q, want api", call.Job)
	}
	if !call.Attached {
		t.Error("the view was opened without a start sequence to drive")
	}
	if got := daemon.startedJobs(); len(got) != 1 || got[0] != "api" {
		t.Errorf("the daemon was asked to start %v, want [api]", got)
	}
}

func TestRunStartDetachedNeverOpensTheView(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "-d")
	if err != nil {
		t.Fatalf("run start api -d: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("-d opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "api started") {
		t.Errorf("stdout does not report the started service:\n%s", stdout)
	}
	if got := daemon.startedJobs(); len(got) != 1 || got[0] != "api" {
		t.Errorf("the daemon was asked to start %v, want [api]", got)
	}
}

// A task is a foreground command: its output belongs to the scrollback, so it
// runs inline on a terminal that would otherwise have got the view.
func TestRunStartTaskStaysInline(t *testing.T) {
	setupStartProject(t, &fakeDaemon{Answers: map[string][]process.Response{
		"migrate": {
			{Status: process.StatusOutput, Data: []byte("applying 001\n")},
			{Status: process.StatusDone},
		},
	}})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "migrate")
	if err != nil {
		t.Fatalf("run start migrate: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a task opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "applying 001") {
		t.Errorf("the task's output never reached the scrollback:\n%s", stdout)
	}
	if !strings.Contains(stdout, "migrate done") {
		t.Errorf("stdout does not report the finished task:\n%s", stdout)
	}
}

func TestRunStartJSONNeverOpensTheView(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run start api --output json: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("--output json opened the view: %+v", view.calls)
	}
	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionStarted {
		t.Errorf("result = %+v, want api started", result)
	}
}

func TestRunStartWithoutATerminalNeverOpensTheView(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api")
	if err != nil {
		t.Fatalf("run start api: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a pipe opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "api started") {
		t.Errorf("stdout does not report the started service:\n%s", stdout)
	}
}
