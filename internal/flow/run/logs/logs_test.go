package logs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func board(views ...runlogs.JobView) runlogs.Board {
	return runlogstest.NewBoard(runlogstest.BoardParams{Views: views})
}

func attachable(name, dir, branch string) runlogs.JobView {
	return runlogs.JobView{
		Name: name, WorkDir: dir, Worktree: branch,
		Status: domain.JobStatusRunning, Attachable: true,
	}
}

// --job names a job, and a board covering three worktrees holds three of them.
// Answering with the first would show one worktree's output under a flag that
// said nothing about which.
func TestViewsTakesEveryWorktreesCopyOfTheNamedJob(t *testing.T) {
	views, err := logsflow.Views(logsflow.ViewsParams{
		Board: board(
			attachable("web", "/work/main", "main"),
			attachable("api", "/work/main", "main"),
			attachable("web", "/work/feature", "feature"),
		),
		Job: "web",
	})
	if err != nil {
		t.Fatalf("Views: %v", err)
	}

	if len(views) != 2 {
		t.Fatalf("views = %d, want one per worktree holding the job", len(views))
	}
	if views[0].WorkDir == views[1].WorkDir {
		t.Error("the two views share a worktree, so they are the same job twice")
	}
}

func TestViewsRefusesAJobNoWorktreeHolds(t *testing.T) {
	_, err := logsflow.Views(logsflow.ViewsParams{
		Board: board(attachable("web", "/work/main", "main")),
		Job:   "nope",
	})

	if err == nil {
		t.Fatal("a job no worktree holds was accepted")
	}
}
