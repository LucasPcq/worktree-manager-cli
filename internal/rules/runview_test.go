package rules

import (
	"fmt"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The four regions tile the screen exactly: every row belongs to one of them,
// and none is placed past the last one.
func TestComputeRunViewLayoutTilesTheScreen(t *testing.T) {
	sizes := []struct{ width, height int }{
		{120, 40}, {100, 24}, {80, 20}, {60, 12}, {40, 8}, {20, 4}, {10, 2}, {4, 1}, {0, 0},
	}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			layout := ComputeRunViewLayout(RunViewLayoutParams{Width: size.width, Height: size.height})

			rows := layout.Header.Height + layout.Notice.Height + layout.Pane.Height + layout.Help.Height
			if rows != size.height {
				t.Fatalf("rows = %d, want the terminal's %d", rows, size.height)
			}
			if got := layout.Sidebar.Width + layout.Pane.Width; got != size.width {
				t.Fatalf("columns = %d, want the terminal's %d", got, size.width)
			}
			if layout.Sidebar.Height != layout.Pane.Height || layout.Sidebar.Y != layout.Pane.Y {
				t.Fatalf("the list and the pane do not share the body: %+v vs %+v", layout.Sidebar, layout.Pane)
			}
			if layout.PaneCols < 0 || layout.PaneRows < 0 || layout.SidebarRows < 0 {
				t.Fatalf("negative content budget: %+v", layout)
			}
		})
	}
}

func TestComputeRunViewLayoutDropsTheListBeforeTheOutput(t *testing.T) {
	wide := ComputeRunViewLayout(RunViewLayoutParams{Width: domain.RunViewSidebarWidth + domain.RunViewSidebarMinPaneCols, Height: 20})
	if !wide.SidebarVisible || wide.Sidebar.Width != domain.RunViewSidebarWidth {
		t.Fatalf("sidebar = %+v, want it shown at its full width", wide.Sidebar)
	}

	narrow := ComputeRunViewLayout(RunViewLayoutParams{Width: domain.RunViewSidebarWidth + domain.RunViewSidebarMinPaneCols - 1, Height: 20})
	if narrow.SidebarVisible || narrow.Sidebar.Width != 0 {
		t.Fatalf("sidebar = %+v, want it dropped rather than squeezing the pane", narrow.Sidebar)
	}
	if narrow.Pane.X != 0 || narrow.Pane.Width != domain.RunViewSidebarWidth+domain.RunViewSidebarMinPaneCols-1 {
		t.Fatalf("pane = %+v, want the whole width once the list is gone", narrow.Pane)
	}
}

// A report is served out of what the body can spare, never out of the pane's
// last rows: a terminal with nothing left keeps its output and drops the band.
func TestComputeRunViewLayoutServesTheNoticeFromTheBody(t *testing.T) {
	roomy := ComputeRunViewLayout(RunViewLayoutParams{Width: 100, Height: 24, NoticeLines: 4})
	if roomy.Notice.Height != 4 {
		t.Fatalf("notice = %d rows, want the 4 it asked for", roomy.Notice.Height)
	}
	if roomy.Pane.Y != roomy.Header.Height+4 {
		t.Fatalf("pane starts at %d, want it under the notice band", roomy.Pane.Y)
	}
	if roomy.Pane.Height != 24-1-1-4 {
		t.Fatalf("pane height = %d, want what the notice left", roomy.Pane.Height)
	}

	tight := ComputeRunViewLayout(RunViewLayoutParams{Width: 100, Height: 6, NoticeLines: 10})
	if tight.Pane.Height < domain.RunViewMinBodyRows {
		t.Fatalf("pane height = %d, want the body to keep %d rows", tight.Pane.Height, domain.RunViewMinBodyRows)
	}
	if tight.Notice.Height != 6-1-1-domain.RunViewMinBodyRows {
		t.Fatalf("notice = %d rows, want only what the body could spare", tight.Notice.Height)
	}
}

func TestComputeRunViewLayoutSizesTheEmulatorToItsBox(t *testing.T) {
	layout := ComputeRunViewLayout(RunViewLayoutParams{Width: 120, Height: 40})

	if want := layout.Pane.Width - domain.RunViewBorderWidth; layout.PaneCols != want {
		t.Fatalf("PaneCols = %d, want the box's inside %d", layout.PaneCols, want)
	}
	if want := layout.Pane.Height - domain.RunViewPanelChrome; layout.PaneRows != want {
		t.Fatalf("PaneRows = %d, want what is left under the title row: %d", layout.PaneRows, want)
	}
	if layout.SidebarRows != layout.Sidebar.Height-domain.RunViewPanelChrome {
		t.Fatalf("SidebarRows = %d, want the rows under the list's title", layout.SidebarRows)
	}
}

func TestJobMark(t *testing.T) {
	cases := []struct {
		label  string
		params JobMarkParams
		want   domain.JobMark
	}{
		{
			label:  "no sequence has spoken",
			params: JobMarkParams{Status: domain.JobStatusRunning},
			want:   domain.JobMarkRunning,
		},
		{
			label:  "the sequence is ahead of the daemon",
			params: JobMarkParams{Status: domain.JobStatusStopped, Step: domain.JobStepStarting, Tracked: true},
			want:   domain.JobMarkStarting,
		},
		{
			label:  "a task the sequence ran to the end",
			params: JobMarkParams{Status: domain.JobStatusStopped, Step: domain.JobStepDone, Tracked: true},
			want:   domain.JobMarkDone,
		},
		{
			label:  "the job that ended the sequence",
			params: JobMarkParams{Status: domain.JobStatusStopped, Step: domain.JobStepFailed, Tracked: true},
			want:   domain.JobMarkCrashed,
		},
		{
			label:  "a job the sequence started and left running",
			params: JobMarkParams{Status: domain.JobStatusRunning, Step: domain.JobStepStarted, Tracked: true},
			want:   domain.JobMarkRunning,
		},
		{
			label:  "one it started that has crashed since",
			params: JobMarkParams{Status: domain.JobStatusCrashed, Step: domain.JobStepStarted, Tracked: true},
			want:   domain.JobMarkCrashed,
		},
		{
			label:  "one it started that has stopped since",
			params: JobMarkParams{Status: domain.JobStatusStopped, Step: domain.JobStepStarted, Tracked: true},
			want:   domain.JobMarkStopped,
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := JobMark(tc.params); got != tc.want {
				t.Fatalf("JobMark(%+v) = %v, want %v", tc.params, got, tc.want)
			}
		})
	}
}

func TestClipReport(t *testing.T) {
	report := []string{"aborted", "failed at 2/3", "left running: api", "not started: web", "esc dismisses"}

	cases := []struct {
		label  string
		height int
		want   []string
	}{
		{label: "every line fits", height: 5, want: report},
		{label: "more room than lines", height: 9, want: report},
		{label: "the way out is kept", height: 3, want: []string{"aborted", "failed at 2/3", "esc dismisses"}},
		{label: "two rows", height: 2, want: []string{"aborted", "esc dismisses"}},
		{label: "one row says what happened", height: 1, want: []string{"aborted"}},
		{label: "no room", height: 0, want: report},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := ClipReport(report, tc.height)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ClipReport(_, %d) = %v, want %v", tc.height, got, tc.want)
			}
			if report[1] != "failed at 2/3" || report[4] != "esc dismisses" {
				t.Fatalf("ClipReport wrote through its argument: %v", report)
			}
		})
	}
}

func TestMatchesJobFilter(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		want   bool
	}{
		{name: "web", filter: "", want: true},
		{name: "web", filter: "  ", want: true},
		{name: "webhook", filter: "WEB", want: true},
		{name: "WEB", filter: "web", want: true},
		{name: "api", filter: "web", want: false},
		{name: "api-gateway", filter: "gate", want: true},
		{name: "api", filter: " api ", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.filter, func(t *testing.T) {
			if got := MatchesJobFilter(tc.name, tc.filter); got != tc.want {
				t.Fatalf("MatchesJobFilter(%q, %q) = %v, want %v", tc.name, tc.filter, got, tc.want)
			}
		})
	}
}
