package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func detachedRecord() domain.JobRecord {
	return domain.JobRecord{
		Name:    "db",
		WorkDir: "/wt/feature",
		Config: domain.JobConfig{
			Name: "db",
			Kind: domain.JobKindService,
			Cmd:  "docker compose up -d",
			Stop: "docker compose down",
		},
	}
}

func TestReconcileJobAdoptsDetachedService(t *testing.T) {
	decision := rules.ReconcileJob(rules.ReconcileJobParams{
		Record:        detachedRecord(),
		WorkDirExists: true,
	})

	if !decision.Adopt {
		t.Fatal("a detached service must be adopted: its stack is still running")
	}
	if decision.Status != domain.JobStatusDetached {
		t.Fatalf("status = %q, want %q", decision.Status, domain.JobStatusDetached)
	}
}

func TestReconcileJobDropsVanishedWorktree(t *testing.T) {
	decision := rules.ReconcileJob(rules.ReconcileJobParams{
		Record:        detachedRecord(),
		WorkDirExists: false,
	})

	if decision.Adopt {
		t.Fatal("a worktree that no longer exists cannot have its stop command run in it")
	}
}

func TestReconcileJobReportsForegroundServiceAsCrashed(t *testing.T) {
	record := detachedRecord()
	record.Config.Stop = ""
	record.Config.Cmd = "pnpm dev"

	decision := rules.ReconcileJob(rules.ReconcileJobParams{
		Record:        record,
		WorkDirExists: true,
	})

	if !decision.Adopt {
		t.Fatal("a foreground service must still be adopted, so run ps and run logs can report it")
	}
	if decision.Status != domain.JobStatusCrashed {
		t.Fatalf("status = %q, want %q: its PTY died with the daemon", decision.Status, domain.JobStatusCrashed)
	}
}

func TestReconcileJobDropsTask(t *testing.T) {
	record := detachedRecord()
	record.Config.Kind = domain.JobKindTask
	record.Config.Stop = ""

	decision := rules.ReconcileJob(rules.ReconcileJobParams{
		Record:        record,
		WorkDirExists: true,
	})

	if decision.Adopt {
		t.Fatal("a task is transient and is never carried across a daemon")
	}
}

func TestIsJobUpKeepsOnlyLiveEntries(t *testing.T) {
	cases := []struct {
		status domain.JobStatus
		want   bool
	}{
		{domain.JobStatusRunning, true},
		{domain.JobStatusDetached, true},
		{domain.JobStatusStopped, false},
		{domain.JobStatusCrashed, false},
	}

	for _, tc := range cases {
		if got := rules.IsJobUp(tc.status); got != tc.want {
			t.Errorf("IsJobUp(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
