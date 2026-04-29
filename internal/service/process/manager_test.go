package process

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestStopKillsChildProcessTree verifies that Stop on a service tears down
// the entire process tree, not just the direct child. This is the regression
// test for the bug where `npm run dev` was killed but the node server it
// spawned was left running.
//
// The shell launched here mimics that pattern: it backgrounds a long-lived
// `sleep` (the "node server") and writes its PID to a marker file, then
// waits. Stop must reap both the shell AND the backgrounded sleep.
func TestStopKillsChildProcessTree(t *testing.T) {
	workDir := t.TempDir()
	markerFile := filepath.Join(workDir, "child.pid")

	scriptPath := writeScript(t, workDir, `#!/bin/sh
sleep 60 &
echo $! > `+markerFile+`
wait
`)

	job := domain.JobConfig{
		Name: "tree",
		Kind: domain.JobKindService,
		Cmd:  "sh " + scriptPath,
	}

	mgr := NewManager()
	if err := mgr.Start(job, workDir, nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	childPID := waitForChildPID(t, markerFile)
	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("child %d should be alive before Stop: %v", childPID, err)
	}

	if err := mgr.Stop("tree", workDir); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if alive(childPID) {
		t.Fatalf("child %d still alive after Stop — process tree was not killed", childPID)
	}
}

// TestStopEscalatesToSIGKILL verifies that a process group ignoring SIGTERM
// is force-killed once the grace period elapses.
func TestStopEscalatesToSIGKILL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 5s SIGKILL escalation test in -short mode")
	}

	workDir := t.TempDir()
	markerFile := filepath.Join(workDir, "child.pid")

	// `trap '' TERM` makes the shell ignore SIGTERM. The backgrounded
	// sleep stays in the same group, so reaping it requires the PG-wide
	// SIGKILL escalation to fire.
	scriptPath := writeScript(t, workDir, `#!/bin/sh
trap '' TERM
sleep 120 &
echo $! > `+markerFile+`
wait
`)

	job := domain.JobConfig{
		Name: "stubborn",
		Kind: domain.JobKindService,
		Cmd:  "sh " + scriptPath,
	}

	mgr := NewManager()
	if err := mgr.Start(job, workDir, nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	childPID := waitForChildPID(t, markerFile)

	start := time.Now()
	if err := mgr.Stop("stubborn", workDir); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < stopGracePeriod {
		t.Errorf("Stop returned in %v, expected >= grace period %v", elapsed, stopGracePeriod)
	}
	if alive(childPID) {
		t.Fatalf("child %d still alive after SIGKILL escalation", childPID)
	}
}

func writeScript(t *testing.T, dir string, body string) string {
	t.Helper()
	path := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func waitForChildPID(t *testing.T, markerFile string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(markerFile)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child pid never written to %s", markerFile)
	return 0
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
