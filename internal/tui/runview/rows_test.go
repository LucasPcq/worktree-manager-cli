package runview

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func inWorktree(view runlogs.JobView, dir, branch string) runlogs.JobView {
	view.WorkDir, view.Worktree = dir, branch
	return view
}

func multiModel(t *testing.T) Model {
	t.Helper()
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
		inWorktree(running("api"), "/work/main", "main"),
		inWorktree(running("web"), "/work/feature", "feature"),
	}})
	model := New(Params{Board: board})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, exec(t, model.refreshCmd()))
}

func TestTheListGroupsJobsUnderTheirWorktree(t *testing.T) {
	rows := multiModel(t).rows()

	if len(rows) != 5 {
		t.Fatalf("rows = %d, want three jobs under two headings", len(rows))
	}
	if rows[0].Header != "main" || rows[3].Header != "feature" {
		t.Fatalf("headings landed at %q and %q", rows[0].Header, rows[3].Header)
	}
	for _, index := range []int{1, 2, 4} {
		if rows[index].Header != "" {
			t.Errorf("row %d is a heading, want a job", index)
		}
	}
}

// The heading is what tells two jobs called `web` apart; below a single
// worktree it would only repeat what the command was told.
func TestTheListStaysFlatAboveOneWorktree(t *testing.T) {
	h := newHarness(t, harnessParams{Views: []runlogs.JobView{running("api"), running("web")}})

	for _, row := range h.model.rows() {
		if row.Header != "" {
			t.Fatalf("a single worktree got a heading: %q", row.Header)
		}
	}
}

// Headings are not selectable: they are a level of the list, not a target.
func TestMovingSkipsTheWorktreeHeadings(t *testing.T) {
	model := multiModel(t)

	seen := map[jobKey]bool{}
	for range 3 {
		seen[model.selected] = true
		next, _ := model.move(1)
		model = next.(Model)
	}
	if len(seen) != 3 {
		t.Fatalf("moving reached %d jobs, want each of them once", len(seen))
	}
}

// Two worktrees running the same profile hold two jobs called `web`. Selecting
// one by name would select both — the same pane and the same subscription.
func TestTheTwoJobsOfTheSameNameAreDistinctTargets(t *testing.T) {
	model := multiModel(t)

	first := model.selected
	for range 2 {
		next, _ := model.move(1)
		model = next.(Model)
	}
	if model.selected == first {
		t.Fatal("the cursor came back to the first job")
	}
	if model.selected.job() != first.job() {
		t.Fatalf("selected %q then %q, want the two jobs of the same name", first.job(), model.selected.job())
	}
	if model.selected.workDir() == first.workDir() {
		t.Fatal("the two selections share a worktree, so they are the same job")
	}
}

func TestTheHeaderCountsTheWorktrees(t *testing.T) {
	view := ansi.Strip(multiModel(t).View())

	if !strings.Contains(view, "2 worktrees") {
		t.Errorf("header does not count the worktrees:\n%s", view)
	}
}

// A run over several worktrees ends several times, and one of them ending says
// nothing about the others.
func TestTheSequenceEndsOnlyWhenEveryWorktreeHasReported(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
		inWorktree(running("web"), "/work/feature", "feature"),
	}})
	model := New(Params{
		Board:     board,
		Worktrees: []string{"main", "feature"},
		Start:     func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return nil, nil },
		Profile:   "dev",
	})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseStarting, Job: "web", WorkDir: "/work/main", Worktree: "main", Step: 1, Steps: 1,
	}})
	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseReady, WorkDir: "/work/main", Worktree: "main",
		Outcome: runlogs.Outcome{WorkDir: "/work/main", Worktree: "main", Steps: 1, Started: []string{"web"}},
	}})
	if !model.sequence.active {
		t.Fatal("the sequence ended although one worktree had not reported")
	}

	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseReady, WorkDir: "/work/feature", Worktree: "feature",
		Outcome: runlogs.Outcome{WorkDir: "/work/feature", Worktree: "feature", Steps: 1, Started: []string{"web"}},
	}})
	if model.sequence.active {
		t.Fatal("the sequence is still running with every worktree reported")
	}
	if len(model.sequence.outcomes) != 2 {
		t.Fatalf("outcomes = %d, want one per worktree", len(model.sequence.outcomes))
	}
}

// They abort independently, so naming one would hide the rest.
func TestTheReportNamesEveryWorktreeThatAborted(t *testing.T) {
	model := multiModel(t)
	model.started = true
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/main", Worktree: "main", Steps: 2, Failed: "web", FailedStep: 1,
	})
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/feature", Worktree: "feature", Steps: 2, Failed: "api", FailedStep: 2,
	})

	report := strings.Join(model.report(), "\n")
	for _, want := range []string{domain.RunViewAbortTitle, "main", "feature"} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}
}
