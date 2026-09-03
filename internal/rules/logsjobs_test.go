package rules

import "testing"

func window(jobs []string, current string, budget int) LogsJobRange {
	return LogsJobWindow(LogsJobWindowParams{Jobs: jobs, Current: current, Budget: budget, Gap: 3, Marks: 2})
}

func TestLogsJobWindowShowsEverythingThatFits(t *testing.T) {
	got := window([]string{"web", "api"}, "web", 60)

	if got.Start != 0 || got.End != 2 {
		t.Errorf("window = %+v, want every job shown when there is room", got)
	}
}

func TestLogsJobWindowAlwaysKeepsTheCurrentJob(t *testing.T) {
	jobs := []string{"web", "api", "pg", "worker", "mailer", "cron"}

	got := window(jobs, "cron", 20)

	if got.Start > 5 || got.End <= 5 {
		t.Errorf("window = %+v, want the current job inside it", got)
	}
	if got.End-got.Start == len(jobs) {
		t.Errorf("window = %+v, want it narrowed to the budget", got)
	}
}

// A budget too small for even one chip still yields that chip: showing none
// would leave the line saying nothing about where the reader is.
func TestLogsJobWindowNeverComesBackEmpty(t *testing.T) {
	got := window([]string{"web", "api"}, "api", 1)

	if got.End-got.Start != 1 || got.Start != 1 {
		t.Errorf("window = %+v, want the current job alone", got)
	}
}

func TestLogsJobWindowIsEmptyWithoutJobs(t *testing.T) {
	if got := window(nil, "", 40); got.End != 0 {
		t.Errorf("window = %+v, want nothing", got)
	}
}

// The jobs after the current one are the ones a reader has not seen yet.
func TestLogsJobWindowGrowsRightFirst(t *testing.T) {
	jobs := []string{"aa", "bb", "cc", "dd", "ee"}

	// Room for the current chip, one more, and the mark on each side — the
	// window is mid-list either way, so both marks are paid for.
	got := window(jobs, "cc", chipWidth("cc")+3+chipWidth("dd")+2+2)

	if got.Start != 2 || got.End != 4 {
		t.Errorf("window = %+v, want it to take the job after the current one", got)
	}
}
