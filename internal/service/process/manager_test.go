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
