package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestManagerStartTask_StreamsOutput verifies that a one-shot task streams its
// output to the provided streamer, returns no error on a clean exit, and is
// removed from the manager afterwards.
func TestManagerStartTask_StreamsOutput(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected streamed output to contain %q, got %q", "hello", buf.String())
	}
	if len(m.List()) != 0 {
		t.Errorf("expected task to be removed after exit, still have %d job(s)", len(m.List()))
	}
}

// TestManagerStartService_AlreadyRunning verifies the daemon contract the CLI
// relies on: starting a service that is already running returns an error whose
// message ends with domain.JobAlreadyRunningSuffix, so `run up` can treat a
// repeat start as a benign no-op instead of aborting the profile.
func TestManagerStartService_AlreadyRunning(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	err := m.Start(StartParams{Job: job, WorkDir: dir})
	if err == nil {
		t.Fatal("expected error starting an already-running service")
	}
	if !strings.Contains(err.Error(), domain.JobAlreadyRunningSuffix) {
		t.Errorf("expected error to contain %q, got %v", domain.JobAlreadyRunningSuffix, err)
	}
}

// TestManagerStartDetached_StreamsOutput verifies that a detached launcher
// (a service with a Stop command, e.g. docker compose up -d) mirrors its
// startup output to the provided streamer live, and stays registered as
// running after the launcher process exits.
func TestManagerStartDetached_StreamsOutput(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo creating-container", Stop: "echo down"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	if !strings.Contains(buf.String(), "creating-container") {
		t.Errorf("expected streamed launcher output to contain %q, got %q", "creating-container", buf.String())
	}
	jobs := m.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusDetached {
		t.Errorf("expected the launcher's exit to leave the job registered as detached, got %+v", jobs)
	}
}

// TestManagerStartDetached_FailureConciseWhenStreamed verifies that a failing
// detached launcher streams its output live and returns a CONCISE error (the
// capture is not re-embedded, since the client already saw it), and is removed.
func TestManagerStartDetached_FailureConciseWhenStreamed(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "up.sh")
	if err := os.WriteFile(script, []byte("echo pull-failed\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "sh " + script, Stop: "echo down"}

	err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf})
	if err == nil {
		t.Fatal("expected error for failing detached launcher")
	}
	if !strings.Contains(buf.String(), "pull-failed") {
		t.Errorf("expected streamed output to contain %q, got %q", "pull-failed", buf.String())
	}
	if strings.Contains(err.Error(), "pull-failed") {
		t.Errorf("expected concise error without re-embedded output, got %v", err)
	}
	if len(m.List()) != 0 {
		t.Errorf("expected failed detached job to be removed, still have %d", len(m.List()))
	}
}

// TestManagerStartDetached_FailureEmbedsOutputWhenNotStreamed verifies that
// without a streamer (e.g. JSON mode) the captured output is embedded in the
// error so the failure reason still reaches the caller.
func TestManagerStartDetached_FailureEmbedsOutputWhenNotStreamed(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "up.sh")
	if err := os.WriteFile(script, []byte("echo pull-failed\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "sh " + script, Stop: "echo down"}

	err := m.Start(StartParams{Job: job, WorkDir: dir})
	if err == nil {
		t.Fatal("expected error for failing detached launcher")
	}
	if !strings.Contains(err.Error(), "pull-failed") {
		t.Errorf("expected error to embed captured output, got %v", err)
	}
}

// TestManagerStartTask_FailureExitCode verifies that a failing task streams its
// output AND returns a concise error carrying the real exit code (the captured
// block is omitted because the streamer already saw it live).
func TestManagerStartTask_FailureExitCode(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "boom.sh")
	if err := os.WriteFile(script, []byte("echo boom\nexit 3\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "failer", Kind: domain.JobKindTask, Cmd: "sh " + script}

	err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf})
	if err == nil {
		t.Fatal("expected error for failing task")
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("expected streamed output to contain %q, got %q", "boom", buf.String())
	}
	if !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("expected error to carry exit code 3, got %v", err)
	}
	// The captured output must not be re-embedded when it was streamed live.
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("expected concise error without re-embedded output, got %v", err)
	}
}

func logDirFor(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "logs", "feat")
}

func waitForLogLine(t *testing.T, path string, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		content, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(content), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("log %s never contained %q (last read: %q, %v)", path, want, content, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestManagerStartTask_PersistsLog verifies that a task's output lands in its
// log file, sanitized and timestamped, on top of being streamed.
func TestManagerStartTask_PersistsLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir, Streamer: &buf}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	waitForLogLine(t, filepath.Join(logDir, "greet.log"), "hello")
}

// TestManagerStartService_PersistsLog verifies that a foreground service — the
// only kind whose output is drained in the background — persists it too.
func TestManagerStartService_PersistsLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "echo listening"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	waitForLogLine(t, filepath.Join(logDir, "server.log"), "listening")
}

// TestManagerStartDetached_PersistsLauncherLog verifies that a detached
// launcher, which has no output hub at all, still gets its startup output on
// disk — that log is all the user will ever have of it.
func TestManagerStartDetached_PersistsLauncherLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo creating-container", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	waitForLogLine(t, filepath.Join(logDir, "compose.log"), "creating-container")
}

// TestManagerStartWithoutLogDir_PersistsNothing pins the opt-in: a client that
// resolved no log dir gets the previous behaviour, no file written.
func TestManagerStartWithoutLogDir_PersistsNothing(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	logs, err := filepath.Glob(filepath.Join(dir, "*"+domain.JobLogFileExt))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("found %v, want no log file", logs)
	}
}

func waitForJob(t *testing.T, m *Manager, name string, until func(ManagedJob) bool) ManagedJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, job := range m.List() {
			if job.Name == name && until(job) {
				return job
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s never reached the expected state (jobs: %+v)", name, m.List())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestManagerStartService_RecordsStartedAt pins what the uptime column is read
// from: the instant the daemon spawned the process, not the instant a client
// asked for the list.
func TestManagerStartService_RecordsStartedAt(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	before := time.Now()
	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	started := m.List()[0]
	if started.StartedAt.Before(before) || started.StartedAt.After(time.Now()) {
		t.Errorf("StartedAt = %v, want between %v and now", started.StartedAt, before)
	}
	if started.ExitCode != nil {
		t.Errorf("ExitCode = %d, want nil while the job runs", *started.ExitCode)
	}
}

// TestManagerService_ReportsExitCodeOnCrash verifies that a service dying on
// its own carries the code it died with, which is what tells a crash apart
// from a clean shutdown once the process is gone.
func TestManagerService_ReportsExitCodeOnCrash(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "boom.sh")
	if err := os.WriteFile(script, []byte("exit 7\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sh " + script}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	crashed := waitForJob(t, m, "server", func(j ManagedJob) bool {
		return j.Status == domain.JobStatusCrashed
	})
	if crashed.ExitCode == nil || *crashed.ExitCode != 7 {
		t.Errorf("ExitCode = %v, want 7", crashed.ExitCode)
	}
}

// TestManagerService_ReportsSignalExitCodeOnStop pins the -1 the JobInfo doc
// promises: a stopped job was killed, it did not choose an exit code.
func TestManagerService_ReportsSignalExitCodeOnStop(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if err := m.Stop("server", dir); err != nil {
		t.Fatalf("stop service: %v", err)
	}

	stopped := m.List()[0]
	if stopped.Status != domain.JobStatusStopped {
		t.Fatalf("Status = %s, want stopped", stopped.Status)
	}
	if stopped.ExitCode == nil || *stopped.ExitCode != -1 {
		t.Errorf("ExitCode = %v, want -1 (killed by a signal)", stopped.ExitCode)
	}
}

// TestManagerStartDetached_KeepsNoExitCode covers the launcher pattern: the
// launcher exiting cleanly says nothing about the service it left running, so
// the job reports no exit code at all rather than a 0 that would read as done.
func TestManagerStartDetached_KeepsNoExitCode(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo up", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	launcher := m.List()[0]
	if launcher.Status != domain.JobStatusDetached {
		t.Fatalf("Status = %s, want detached", launcher.Status)
	}
	if launcher.ExitCode != nil {
		t.Errorf("ExitCode = %d, want nil for a detached launcher", *launcher.ExitCode)
	}
	if launcher.StartedAt.IsZero() {
		t.Error("StartedAt is zero, want the launcher's spawn instant")
	}
}
