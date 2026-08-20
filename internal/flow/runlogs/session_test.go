package runlogs_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

const workDir = "/work/api"

func newSession(t *testing.T, service *runlogstest.Service, jobs ...domain.JobConfig) runlogs.Session {
	t.Helper()
	session := runlogs.NewSession(runlogs.SessionParams{
		Service: service,
		Jobs:    jobs,
		WorkDir: workDir,
		LogDir:  "/state/logs/api",
	})
	if err := session.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return session
}

func running(name string, kind domain.JobKind) domain.JobInfo {
	return domain.JobInfo{
		Name:      name,
		Kind:      kind,
		Status:    domain.JobStatusRunning,
		WorkDir:   workDir,
		StartedAt: time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC),
	}
}

func TestSessionListsDeclaredJobsWhetherOrNotTheyRun(t *testing.T) {
	service := &runlogstest.Service{Infos: []domain.JobInfo{running("api", domain.JobKindService)}}

	views := newSession(t, service, migrate, api, docker).Jobs()

	names := make([]string, len(views))
	for i, view := range views {
		names[i] = view.Name
	}
	if !reflect.DeepEqual(names, []string{"migrate", "api", "docker"}) {
		t.Fatalf("jobs %v, want the declared order", names)
	}
	if views[0].Status != domain.JobStatusStopped || views[0].Attachable {
		t.Fatalf("a job nobody started reads as %+v", views[0])
	}
	if views[1].Status != domain.JobStatusRunning || !views[1].Attachable {
		t.Fatalf("the running service reads as %+v", views[1])
	}
	if views[1].StartedAt.IsZero() {
		t.Fatal("the running service lost its start time")
	}
}

func TestSessionRefusesToAttachAJobWithoutLiveOutput(t *testing.T) {
	exited := 1
	service := &runlogstest.Service{Infos: []domain.JobInfo{
		running("api", domain.JobKindService),
		running("docker", domain.JobKindService),
		{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped, WorkDir: workDir, ExitCode: &exited},
	}}
	session := newSession(t, service, migrate, api, docker)

	for _, job := range []string{"docker", "migrate"} {
		if _, err := session.Attach(runlogs.AttachParams{Job: job}); !errors.Is(err, domain.ErrJobNotAttachable) {
			t.Fatalf("attaching %s: %v, want ErrJobNotAttachable", job, err)
		}
	}
	if _, err := session.Attach(runlogs.AttachParams{Job: "ghost"}); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("attaching an unknown job: %v, want ErrJobNotFound", err)
	}
	if len(service.Attached) > 0 {
		t.Fatalf("the daemon was asked anyway: %+v", service.Attached)
	}

	views := session.Jobs()
	if views[2].Attachable {
		t.Fatal("a detached launcher offers a live stream")
	}
	if views[0].ExitCode == nil || *views[0].ExitCode != 1 {
		t.Fatalf("the stopped task lost its exit code: %+v", views[0])
	}
}

// The daemon streams a task for as long as it runs: newJobHub withholds a hub
// from a detached launcher only, and attachableJob takes any job that has one.
// A three-minute migration read from a second terminal belongs in its pane, not
// in the sanitized file the log falls back to.
func TestSessionAttachesARunningTask(t *testing.T) {
	stream := runlogstest.NewStream()
	service := &runlogstest.Service{
		Infos:   []domain.JobInfo{running("migrate", domain.JobKindTask)},
		Streams: map[string]runlogs.Stream{"migrate": stream},
	}
	session := newSession(t, service, migrate)

	if views := session.Jobs(); !views[0].Attachable {
		t.Fatalf("a running task reads as %+v", views[0])
	}
	if _, err := session.Attach(runlogs.AttachParams{Job: "migrate"}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
}

func TestSessionRefusesToAttachAJobThatStopped(t *testing.T) {
	exited := 0
	stopped := func(name string, kind domain.JobKind) domain.JobInfo {
		return domain.JobInfo{
			Name: name, Kind: kind, Status: domain.JobStatusStopped,
			WorkDir: workDir, ExitCode: &exited,
		}
	}
	service := &runlogstest.Service{Infos: []domain.JobInfo{
		stopped("api", domain.JobKindService),
		stopped("legacy", domain.JobKindService),
		stopped("seed", domain.JobKindTask),
	}}

	views := newSession(t, service, api).Jobs()

	for _, view := range views {
		if view.Attachable {
			t.Fatalf("%s stopped and still offers a live stream: %+v", view.Name, view)
		}
	}
	if len(views) != 3 {
		t.Fatalf("jobs %+v, want the declared service and the two the daemon still holds", views)
	}
}

// A job run.toml no longer declares is read the same way, task or service: what
// the daemon holds a hub for is what can be attached.
func TestSessionAttachesARunningUndeclaredTask(t *testing.T) {
	service := &runlogstest.Service{Infos: []domain.JobInfo{running("legacy-migrate", domain.JobKindTask)}}

	views := newSession(t, service, api).Jobs()

	if len(views) != 2 || views[1].Name != "legacy-migrate" {
		t.Fatalf("jobs %+v, want the undeclared task listed", views)
	}
	if !views[1].Attachable {
		t.Fatalf("a running undeclared task reads as %+v", views[1])
	}
}

func TestSessionAttachSizesThePTYToThePane(t *testing.T) {
	stream := runlogstest.NewStream()
	service := &runlogstest.Service{
		Infos:   []domain.JobInfo{running("api", domain.JobKindService)},
		Streams: map[string]runlogs.Stream{"api": stream},
	}
	session := newSession(t, service, api)

	attached, err := session.Attach(runlogs.AttachParams{Job: "api", Size: runlogs.Size{Cols: 120, Rows: 40}})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if attached != runlogs.Stream(stream) {
		t.Fatal("Attach returned another stream")
	}

	want := runlogs.AttachRequest{Name: "api", WorkDir: workDir, Size: runlogs.Size{Cols: 120, Rows: 40}}
	if !reflect.DeepEqual(service.Attached, []runlogs.AttachRequest{want}) {
		t.Fatalf("attach requests %+v, want %+v", service.Attached, want)
	}
}

func TestSessionReadsAStoppedJobsHistoryFromItsLog(t *testing.T) {
	service := &runlogstest.Service{Lines: map[string][]string{"migrate": {"2026-08-20T10:00:00Z  applying 001"}}}
	session := newSession(t, service, migrate)

	lines, err := session.History(runlogs.HistoryParams{Job: "migrate", Lines: 50})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if !reflect.DeepEqual(lines, []string{"2026-08-20T10:00:00Z  applying 001"}) {
		t.Fatalf("history %v", lines)
	}

	want := runlogs.TailRequest{LogDir: "/state/logs/api", Job: "migrate", Lines: 50}
	if !reflect.DeepEqual(service.Tailed, []runlogs.TailRequest{want}) {
		t.Fatalf("tail requests %+v, want %+v", service.Tailed, want)
	}

	if _, err := session.History(runlogs.HistoryParams{Job: "migrate"}); err != nil {
		t.Fatalf("History without a count: %v", err)
	}
	if got := service.Tailed[1].Lines; got != domain.JobLogTailLines {
		t.Fatalf("default line count = %d, want %d", got, domain.JobLogTailLines)
	}
}

func TestSessionIgnoresAnotherWorktreesJobsAndKeepsUndeclaredOnes(t *testing.T) {
	other := running("api", domain.JobKindService)
	other.WorkDir = "/work/web"
	service := &runlogstest.Service{Infos: []domain.JobInfo{other, running("legacy", domain.JobKindService)}}

	views := newSession(t, service, api).Jobs()

	if len(views) != 2 {
		t.Fatalf("jobs %+v, want the declared one and the undeclared runner", views)
	}
	if views[0].Name != "api" || views[0].Status != domain.JobStatusStopped {
		t.Fatalf("another worktree's job leaked into %+v", views[0])
	}
	if views[1].Name != "legacy" || !views[1].Attachable {
		t.Fatalf("a running job run.toml no longer declares reads as %+v", views[1])
	}
}

func TestSessionRefreshSurfacesTheDaemonsError(t *testing.T) {
	service := &runlogstest.Service{ListErr: errors.New("connect to daemon: no such file")}
	session := runlogs.NewSession(runlogs.SessionParams{Service: service, Jobs: []domain.JobConfig{api}, WorkDir: workDir})

	if err := session.Refresh(); err == nil {
		t.Fatal("Refresh swallowed the daemon error")
	}
	if views := session.Jobs(); len(views) != 1 || views[0].Status != domain.JobStatusStopped {
		t.Fatalf("jobs after a failed refresh = %+v", views)
	}
}
