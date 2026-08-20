package runview

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func resize(model Model, width, height int) Model {
	return update(model, tea.WindowSizeMsg{Width: width, Height: height})
}

// The frame is what is written to a terminal holding nothing else: one line per
// row, none wider than the screen. A pane drawn one column too wide wraps every
// row under it and the whole view shears.
func TestFrameFitsTheTerminalExactly(t *testing.T) {
	sizes := []struct{ width, height int }{
		{120, 40}, {100, 24}, {80, 20}, {60, 15}, {40, 10}, {20, 6}, {8, 3},
	}
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web"), stopped("migrate")},
		Streams: []string{"api", "web"},
	})
	h.streams["api"].Feed([]byte(strings.Repeat("a wide line of job output\r\n", 60)))
	h.waitForPane(t, "api", "a wide line")

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			frame := resize(h.model, size.width, size.height).View()
			lines := strings.Split(frame, "\n")
			if len(lines) != size.height {
				t.Fatalf("frame has %d lines, want %d", len(lines), size.height)
			}
			for index, line := range lines {
				if width := lipgloss.Width(line); width > size.width {
					t.Fatalf("line %d is %d columns wide, want at most %d: %q", index, width, size.width, line)
				}
			}
		})
	}
}

func TestNarrowTerminalDropsTheJobList(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if !strings.Contains(ansi.Strip(resize(h.model, 100, 24).View()), domain.RunViewJobsTitle) {
		t.Fatal("the job list is missing on a wide terminal")
	}
	if strings.Contains(ansi.Strip(resize(h.model, 50, 24).View()), domain.RunViewJobsTitle) {
		t.Fatal("the job list was kept on a terminal too narrow to hold it beside a pane")
	}
}

func TestJobListCarriesMarkAndUptime(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), stopped("migrate")},
		Streams: []string{"api"},
	})

	frame := ansi.Strip(h.model.View())
	want := []string{
		domain.RunViewCursorMark + domain.RunViewMarkRunning + " api",
		domain.RunViewMarkStopped + " migrate",
		"1m",
	}
	for _, line := range want {
		if !strings.Contains(frame, line) {
			t.Fatalf("frame = %q, want %q", frame, line)
		}
	}
	// Uptime reads a job that is running: a stopped one dates a run that is over.
	if strings.Count(frame, "1m") != 1 {
		t.Fatal("an uptime was rendered for a job that is not running")
	}
}

func TestPaneTitleNamesTheJobAndWhereItsBytesComeFrom(t *testing.T) {
	live := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	if got := ansi.Strip(live.model.View()); !strings.Contains(got, "api · running") ||
		!strings.Contains(got, domain.RunViewPaneLiveLabel) {
		t.Fatalf("frame = %q, want the job, its status and its live origin", got)
	}

	history := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("api")},
		Lines: map[string][]string{"api": {"bye"}},
	})
	if got := ansi.Strip(history.model.View()); !strings.Contains(got, domain.RunViewPaneHistoryLabel) {
		t.Fatalf("frame = %q, want the pane to say it is showing the log file", got)
	}
}

func TestFooterSwitchesToTheFilterPrompt(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if !strings.Contains(ansi.Strip(h.model.View()), domain.RunViewHelpBrowse) {
		t.Fatal("the footer does not carry the browse keys")
	}

	h.press(t, key("/"))
	h.press(t, key("a"))
	footer := ansi.Strip(h.model.View())
	if !strings.Contains(footer, domain.RunViewFilterPrompt+"a") {
		t.Fatalf("footer = %q, want the filter box and what was typed into it", footer)
	}
	if !strings.Contains(footer, domain.RunViewHelpFilter) {
		t.Fatal("the footer does not say how to leave the filter")
	}

	h.press(t, namedKey(tea.KeyEnter))
	if applied := ansi.Strip(h.model.View()); !strings.Contains(applied, "filter \"a\"") {
		t.Fatalf("footer = %q, want an applied filter to keep saying it is on", applied)
	}
}

// The emulator's whole point is that a job's colours survive; the frame must not
// strip or reset them on the way out.
func TestFrameKeepsTheJobsTruecolor(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	h.streams["api"].Feed([]byte("\x1b[38;2;255;0;0mfailed\x1b[0m\r\n"))
	h.waitForPane(t, "api", "failed")

	if !strings.Contains(h.model.View(), "38;2;255;0;0") {
		t.Fatal("the pane's truecolor was lost in the frame")
	}
}

// A pane is fed at the size it is drawn at: an emulator wider than its box
// pushes the border off the row.
func TestPanesAreSizedFromTheLayout(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	model := resize(h.model, 120, 40)
	layout := model.layout()
	entry, held := model.panes.entry("api")
	if !held {
		t.Fatal("the selected job has no pane")
	}
	if got := entry.pane.Size(); got.Cols != layout.PaneCols || got.Rows != layout.PaneRows {
		t.Fatalf("pane size = %+v, want the layout's %dx%d", got, layout.PaneCols, layout.PaneRows)
	}
}
