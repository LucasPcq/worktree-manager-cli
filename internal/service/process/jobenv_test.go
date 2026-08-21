package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobEnvOverridesBeatWhatTheDaemonInherited(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv(domain.EnvComposeProjectName, "inherited-from-another-worktree")

	env := jobEnv(jobEnvParams{
		Kind: domain.JobKindService,
		Overrides: map[string]string{
			domain.EnvComposeProjectName: "feat-x",
			domain.EnvOrdinal:            "2",
		},
	})

	if got, _ := lookupEnv(env, domain.EnvComposeProjectName); got != "feat-x" {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, "feat-x")
	}
	if got, _ := lookupEnv(env, domain.EnvOrdinal); got != "2" {
		t.Errorf("%s = %q, want %q", domain.EnvOrdinal, got, "2")
	}
}

// The task branch forces TERM=dumb. An override is applied before that, so a
// worktree cannot break the terminal contract, and TERM stays dumb.
func TestJobEnvKeepsTaskTerminalContract(t *testing.T) {
	env := jobEnv(jobEnvParams{
		Kind:      domain.JobKindTask,
		Overrides: map[string]string{"TERM": "xterm-256color"},
	})

	if got, _ := lookupEnv(env, "TERM"); got != "dumb" {
		t.Errorf("TERM = %q, want %q", got, "dumb")
	}
}

func TestJobEnvWithoutOverridesIsUnchanged(t *testing.T) {
	if len(jobEnv(jobEnvParams{Kind: domain.JobKindService})) != len(jobEnv(jobEnvParams{Kind: domain.JobKindService, Overrides: map[string]string{}})) {
		t.Error("an empty override map changed the environment")
	}
}

func TestManagerStartTask_InjectsWorktreeEnv(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "env", Kind: domain.JobKindTask, Cmd: "printenv " + domain.EnvPortOffset}

	if err := m.Start(StartParams{
		Job:      job,
		WorkDir:  dir,
		Env:      map[string]string{domain.EnvPortOffset: "70"},
		Streamer: &buf,
	}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if !strings.Contains(buf.String(), "70") {
		t.Errorf("task did not see %s=70, got %q", domain.EnvPortOffset, buf.String())
	}
}

// A detached service's stop command must run in the same environment its start
// did: a `docker compose down` that lost COMPOSE_PROJECT_NAME tears down the
// wrong project, or nothing at all.
func TestManagerStop_RunsStopCommandWithJobEnv(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	marker := filepath.Join(dir, "stopped-project")

	// A script rather than an inline command: the daemon splits a job's cmd on
	// whitespace, so a redirection has nowhere to live (LUC-196).
	script := filepath.Join(dir, "stop.sh")
	body := "#!/bin/sh\nprintenv " + domain.EnvComposeProjectName + " > " + marker + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write stop script: %v", err)
	}

	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "true", Stop: script}
	if err := m.Start(StartParams{
		Job:     job,
		WorkDir: dir,
		Env:     map[string]string{domain.EnvComposeProjectName: "feat-x"},
	}); err != nil {
		t.Fatalf("start detached service: %v", err)
	}

	if err := m.Stop(job.Name, dir); err != nil {
		t.Fatalf("stop: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("stop command left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "feat-x" {
		t.Errorf("stop command saw %s=%q, want %q", domain.EnvComposeProjectName, strings.TrimSpace(string(got)), "feat-x")
	}
}
