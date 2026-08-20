package runlogs_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var (
	migrate = domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"}
	api     = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}
	docker  = domain.JobConfig{Name: "docker", Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}
)

var phaseNames = map[runlogs.Phase]string{
	runlogs.PhaseStarting: "starting",
	runlogs.PhaseOutput:   "output",
	runlogs.PhaseStarted:  "started",
	runlogs.PhaseDone:     "done",
	runlogs.PhaseFailed:   "failed",
	runlogs.PhaseAborted:  "aborted",
	runlogs.PhaseReady:    "ready",
}

func trace(sink *runlogstest.Sink) string {
	steps := make([]string, 0, len(sink.Events))
	for _, event := range sink.Events {
		steps = append(steps, phaseNames[event.Phase]+":"+event.Job)
	}
	return strings.Join(steps, " ")
}

func run(t *testing.T, service *runlogstest.Service, jobs ...domain.JobConfig) (runlogs.Outcome, *runlogstest.Sink) {
	t.Helper()
	sink := &runlogstest.Sink{}
	outcome, err := runlogs.Run(runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    jobs,
		WorkDir: "/work/api",
		LogDir:  "/state/logs/api",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return outcome, sink
}

func TestRunStartsEveryJobInDeclaredOrder(t *testing.T) {
	service := &runlogstest.Service{}

	outcome, sink := run(t, service, migrate, docker, api)

	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"migrate", "docker", "api"}) {
		t.Fatalf("started %v, want the declared order", got)
	}
	if got := trace(sink); got != "starting:migrate done:migrate starting:docker started:docker starting:api started:api ready:" {
		t.Fatalf("trace = %q", got)
	}
	if !reflect.DeepEqual(outcome.Completed, []string{"migrate"}) {
		t.Fatalf("completed %v, want the task", outcome.Completed)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker", "api"}) {
		t.Fatalf("started %v, want both services", outcome.Started)
	}
	if outcome.Aborted() || outcome.NotStarted != nil {
		t.Fatalf("outcome reports an abort: %+v", outcome)
	}
	if outcome.Steps != 3 {
		t.Fatalf("steps = %d, want 3", outcome.Steps)
	}

	want := []domain.JobActionResult{
		{Name: "migrate", Status: domain.JobActionDone},
		{Name: "docker", Status: domain.JobActionStarted},
		{Name: "api", Status: domain.JobActionStarted},
	}
	if !reflect.DeepEqual(outcome.Results, want) {
		t.Fatalf("results %+v, want %+v", outcome.Results, want)
	}

	ready, found := sink.Last(runlogs.PhaseReady)
	if !found {
		t.Fatal("no ready event")
	}
	if !reflect.DeepEqual(ready.Outcome, outcome) {
		t.Fatalf("ready carries %+v, want the returned outcome", ready.Outcome)
	}
}

func TestRunAbortsOnAFailingTaskAndReportsThePartialState(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
	}

	outcome, sink := run(t, service, docker, migrate, api)

	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"docker", "migrate"}) {
		t.Fatalf("started %v, want nothing after the failure", got)
	}
	if outcome.Failed != "migrate" || outcome.FailedStep != 2 || outcome.Steps != 3 {
		t.Fatalf("failure placed at %q %d/%d, want migrate 2/3", outcome.Failed, outcome.FailedStep, outcome.Steps)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker"}) {
		t.Fatalf("left running %v, want docker", outcome.Started)
	}
	if !reflect.DeepEqual(outcome.NotStarted, []string{"api"}) {
		t.Fatalf("not started %v, want api", outcome.NotStarted)
	}
	if outcome.Completed != nil {
		t.Fatalf("completed %v, want nothing", outcome.Completed)
	}

	failed, found := sink.Last(runlogs.PhaseFailed)
	if !found || failed.Job != "migrate" || failed.Reason != "task migrate failed: exit status 1" {
		t.Fatalf("failed event = %+v", failed)
	}

	aborted, found := sink.Last(runlogs.PhaseAborted)
	if !found {
		t.Fatal("no aborted event")
	}
	if !reflect.DeepEqual(aborted.Outcome, outcome) {
		t.Fatalf("abort reports %+v, want the returned outcome", aborted.Outcome)
	}

	last := outcome.Results[len(outcome.Results)-1]
	if last.Status != domain.JobActionError || last.Message != "task migrate failed: exit status 1" {
		t.Fatalf("last result = %+v, want the failure and its reason", last)
	}
}

func TestRunTreatsAnAlreadyRunningLauncherAsStarted(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"docker": "job docker " + domain.JobAlreadyRunningSuffix},
	}

	outcome, sink := run(t, service, docker, api)

	if outcome.Aborted() {
		t.Fatalf("a repeat start aborted the profile: %+v", outcome)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker", "api"}) {
		t.Fatalf("started %v, want both", outcome.Started)
	}

	started, found := sink.Last(runlogs.PhaseStarted)
	if !found {
		t.Fatal("no started event")
	}
	if started.Job != "api" || started.AlreadyRunning {
		t.Fatalf("last started event = %+v, want api started for real", started)
	}
	if sink.Events[1].Job != "docker" || !sink.Events[1].AlreadyRunning {
		t.Fatalf("docker event = %+v, want it marked already running", sink.Events[1])
	}
}

// A task is a step to run, not a state to reach: the daemon refusing it because
// it is already running means it has not run here, so the profile stops.
func TestRunAbortsOnAnAlreadyRunningTask(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "job migrate " + domain.JobAlreadyRunningSuffix},
	}

	outcome, _ := run(t, service, migrate, api)

	if outcome.Failed != "migrate" {
		t.Fatalf("outcome = %+v, want the task to abort the profile", outcome)
	}
}

func TestRunAbortsWhenTheDaemonCannotBeReached(t *testing.T) {
	service := &runlogstest.Service{
		Errors: map[string]error{"api": errors.New("connect to daemon: no such file")},
	}

	outcome, sink := run(t, service, docker, api)

	if outcome.Failed != "api" || outcome.FailedStep != 2 {
		t.Fatalf("outcome = %+v, want api to abort at step 2", outcome)
	}
	failed, _ := sink.Last(runlogs.PhaseFailed)
	if failed.Reason != "connect to daemon: no such file" {
		t.Fatalf("reason = %q, want the transport error", failed.Reason)
	}
}

func TestRunForwardsAJobsOutputAsItStarts(t *testing.T) {
	service := &runlogstest.Service{
		Output: map[string][]string{"migrate": {"applying 001\n", "applying 002\n"}},
	}

	_, sink := run(t, service, migrate)

	if got := trace(sink); got != "starting:migrate output:migrate output:migrate done:migrate ready:" {
		t.Fatalf("trace = %q, want the output before the conclusion", got)
	}
	if got := string(sink.Events[1].Chunk); got != "applying 001\n" {
		t.Fatalf("chunk = %q, want the bytes untouched", got)
	}
	if sink.Events[1].Step != 1 || sink.Events[1].Steps != 1 {
		t.Fatalf("output event = %+v, want it placed in the sequence", sink.Events[1])
	}
}

func TestRunWithoutAServiceIsRefused(t *testing.T) {
	if _, err := runlogs.Run(runlogs.RunParams{Jobs: []domain.JobConfig{api}}); err == nil {
		t.Fatal("Run accepted a nil service")
	}
}

func TestRunWithoutASinkStillRuns(t *testing.T) {
	service := &runlogstest.Service{}

	outcome, err := runlogs.Run(runlogs.RunParams{Service: service, Jobs: []domain.JobConfig{api}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"api"}) {
		t.Fatalf("started %v, want api", outcome.Started)
	}
}
