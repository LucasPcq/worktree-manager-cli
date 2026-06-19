package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	if err := m.Start(job, dir, &buf); err != nil {
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
	if err := m.Start(job, dir, nil); err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	err := m.Start(job, dir, nil)
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

	if err := m.Start(job, dir, &buf); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	if !strings.Contains(buf.String(), "creating-container") {
		t.Errorf("expected streamed launcher output to contain %q, got %q", "creating-container", buf.String())
	}
	jobs := m.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusRunning {
		t.Errorf("expected detached job to stay registered as running, got %+v", jobs)
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

	err := m.Start(job, dir, &buf)
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

	err := m.Start(job, dir, nil)
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

	err := m.Start(job, dir, &buf)
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
