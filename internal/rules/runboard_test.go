package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunBoardGroupsByWorktreeAndSkipsTheIdleOnes(t *testing.T) {
	now := time.Now()
	board := RunBoard(RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/a", StartedAt: now.Add(-time.Minute)},
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/b", StartedAt: now.Add(-time.Hour)},
		},
		Addresses: map[string]map[string]domain.JobAddress{
			"feat/a": {"web": {URL: "http://a.wtm"}},
			"feat/b": {"web": {URL: "http://b.wtm"}},
		},
		Statuses: []domain.WorktreeStatus{
			{Branch: "feat/a", Path: "/wt/a"},
			{Branch: "feat/idle", Path: "/wt/idle"},
			{Branch: "feat/b", Path: "/wt/b"},
		},
		Now: now,
	})

	if len(board) != 2 {
		t.Fatalf("blocks = %d, want only the worktrees running something", len(board))
	}
	if board[0].Branch != "feat/a" || board[1].Branch != "feat/b" {
		t.Errorf("branches = %q, %q, want the statuses' own order", board[0].Branch, board[1].Branch)
	}
	if board[0].Up != 1 {
		t.Errorf("Up = %d, want 1", board[0].Up)
	}
	if len(board[0].Rows) != 1 {
		t.Fatalf("rows = %d, want only what is up: this board answers what runs", len(board[0].Rows))
	}
	if board[0].Rows[0].URL != "http://a.wtm" {
		t.Errorf("URL = %q, want feat/a's own — a job of the same name elsewhere is not this one", board[0].Rows[0].URL)
	}
	if board[0].Path != "/wt/a" {
		t.Errorf("Path = %q, want the worktree's own", board[0].Path)
	}
}

func TestRunBoardIsEmptyWhenNothingRuns(t *testing.T) {
	if got := RunBoard(RunBoardParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Statuses: []domain.WorktreeStatus{{Branch: "feat/a", Path: "/wt/a"}},
		Now:      time.Now(),
	}); len(got) != 0 {
		t.Errorf("board = %v, want empty", got)
	}
}

// A job the daemon holds up but run.toml no longer declares has no row to
// draw: the board reads the declaration, like the detail panel's section.
func TestRunBoardSkipsAJobTheConfigNoLongerDeclares(t *testing.T) {
	now := time.Now()
	board := RunBoard(RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs: []domain.JobInfo{
			{Name: "gone", Status: domain.JobStatusRunning, WorkDir: "/wt/a", StartedAt: now},
		},
		Statuses: []domain.WorktreeStatus{{Branch: "feat/a", Path: "/wt/a"}},
		Now:      now,
	})

	if len(board) != 0 {
		t.Errorf("board = %+v, want no block: nothing declared is up here", board)
	}
}
