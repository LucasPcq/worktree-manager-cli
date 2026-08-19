package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestComputeDashboardLayoutSplitsWideTerminals(t *testing.T) {
	layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40})

	if layout.Narrow {
		t.Fatal("120 columns is above the narrow threshold")
	}
	if !layout.ListVisible || !layout.DetailVisible {
		t.Fatalf("both panels should show, got list=%v detail=%v", layout.ListVisible, layout.DetailVisible)
	}
	if layout.List.Width+layout.Detail.Width != 120 {
		t.Errorf("panels span %d columns, want the full 120", layout.List.Width+layout.Detail.Width)
	}
	if layout.Detail.X != layout.List.Width {
		t.Errorf("detail starts at x=%d, want it flush against the list (%d)", layout.Detail.X, layout.List.Width)
	}
	if layout.List.Y != layout.Tabs.Height {
		t.Errorf("body starts at y=%d, want it right under the tab bar", layout.List.Y)
	}
}

func TestComputeDashboardLayoutNarrowDropsTheDetail(t *testing.T) {
	closed := ComputeDashboardLayout(DashboardLayoutParams{Width: 80, Height: 30})
	if !closed.Narrow || closed.DetailVisible || !closed.ListVisible {
		t.Fatalf("narrow closed = %+v, want list only", closed)
	}
	if closed.List.Width != 80 {
		t.Errorf("list width = %d, want the full width", closed.List.Width)
	}

	opened := ComputeDashboardLayout(DashboardLayoutParams{Width: 80, Height: 30, DetailOpen: true})
	if !opened.DetailVisible || opened.ListVisible {
		t.Fatalf("narrow opened = %+v, want detail only", opened)
	}
	if opened.Detail.Width != 80 {
		t.Errorf("detail width = %d, want the full width", opened.Detail.Width)
	}
}

func TestComputeDashboardLayoutNarrowThresholdIsExclusive(t *testing.T) {
	if ComputeDashboardLayout(DashboardLayoutParams{Width: domain.DashboardNarrowWidth, Height: 30}).Narrow {
		t.Error("exactly the threshold width must stay wide")
	}
	if !ComputeDashboardLayout(DashboardLayoutParams{Width: domain.DashboardNarrowWidth - 1, Height: 30}).Narrow {
		t.Error("one column under the threshold must turn narrow")
	}
}

func TestComputeDashboardLayoutOutputPanelFoldsAndUnfolds(t *testing.T) {
	folded := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40})
	expanded := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40, OutputExpanded: true})

	if folded.OutputLines != 0 {
		t.Errorf("folded output shows %d lines, want none", folded.OutputLines)
	}
	if folded.Output.Height != domain.DashboardChromeHeight {
		t.Errorf("folded output height = %d, want chrome only", folded.Output.Height)
	}
	if expanded.OutputLines != domain.DashboardOutputBodyHeight {
		t.Errorf("expanded output shows %d lines, want %d", expanded.OutputLines, domain.DashboardOutputBodyHeight)
	}
	if expanded.List.Height >= folded.List.Height {
		t.Error("unfolding the output panel must take its rows from the body")
	}
}

func TestComputeDashboardLayoutStacksWithoutOverlapOrGap(t *testing.T) {
	for _, expanded := range []bool{false, true} {
		layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 40, OutputExpanded: expanded})
		if got := layout.List.Y + layout.List.Height; got != layout.Output.Y {
			t.Errorf("expanded=%v: body ends at y=%d, output starts at y=%d", expanded, got, layout.Output.Y)
		}
		if got := layout.Output.Y + layout.Output.Height; got != layout.Help.Y {
			t.Errorf("expanded=%v: output ends at y=%d, help sits at y=%d", expanded, got, layout.Help.Y)
		}
		if layout.Help.Y+layout.Help.Height != 40 {
			t.Errorf("expanded=%v: last row is %d, want 40", expanded, layout.Help.Y+layout.Help.Height)
		}
	}
}

func TestComputeDashboardLayoutShrinksTheOutputPanelBeforeTheBody(t *testing.T) {
	layout := ComputeDashboardLayout(DashboardLayoutParams{Width: 120, Height: 8, OutputExpanded: true})

	if layout.List.Height < minDashboardBody {
		t.Errorf("body height = %d, want at least %d", layout.List.Height, minDashboardBody)
	}
	if layout.Output.Height >= domain.DashboardChromeHeight+domain.DashboardOutputBodyHeight {
		t.Errorf("output height = %d, want it given up to the body", layout.Output.Height)
	}
}

func TestComputeDashboardLayoutSurvivesDegenerateSizes(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {10, 2}, {200, 3}} {
		layout := ComputeDashboardLayout(DashboardLayoutParams{Width: size[0], Height: size[1]})
		for name, rect := range map[string]domain.Rect{
			"tabs": layout.Tabs, "list": layout.List, "detail": layout.Detail,
			"output": layout.Output, "help": layout.Help,
		} {
			if rect.Width < 0 || rect.Height < 0 || rect.X < 0 || rect.Y < 0 {
				t.Errorf("%dx%d: %s = %+v has a negative dimension", size[0], size[1], name, rect)
			}
		}
		if layout.ListRows < 0 || layout.OutputLines < 0 {
			t.Errorf("%dx%d: negative row counts", size[0], size[1])
		}
	}
}

func TestDashboardScrollOffsetKeepsTheCursorVisible(t *testing.T) {
	tests := []struct {
		name   string
		params DashboardScrollParams
		want   int
	}{
		{"everything fits", DashboardScrollParams{Cursor: 3, Total: 5, Visible: 10}, 0},
		{"no room", DashboardScrollParams{Cursor: 3, Total: 5, Visible: 0}, 0},
		{"cursor above the window", DashboardScrollParams{Cursor: 2, Total: 20, Visible: 5, Offset: 7}, 2},
		{"cursor below the window", DashboardScrollParams{Cursor: 12, Total: 20, Visible: 5, Offset: 0}, 8},
		{"cursor inside the window", DashboardScrollParams{Cursor: 9, Total: 20, Visible: 5, Offset: 7}, 7},
		{"offset past the end", DashboardScrollParams{Cursor: 19, Total: 20, Visible: 5, Offset: 99}, 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DashboardScrollOffset(tc.params); got != tc.want {
				t.Errorf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestClampIndex(t *testing.T) {
	tests := []struct{ index, count, want int }{
		{-1, 5, 0}, {0, 5, 0}, {4, 5, 4}, {9, 5, 4}, {3, 0, 0},
	}
	for _, tc := range tests {
		if got := ClampIndex(tc.index, tc.count); got != tc.want {
			t.Errorf("ClampIndex(%d, %d) = %d, want %d", tc.index, tc.count, got, tc.want)
		}
	}
}

func TestDashboardClampOffsetKeepsAFreeScrollInRange(t *testing.T) {
	tests := []struct {
		name   string
		params DashboardOffsetParams
		want   int
	}{
		{"everything fits", DashboardOffsetParams{Offset: 4, Total: 5, Visible: 10}, 0},
		{"no room", DashboardOffsetParams{Offset: 4, Total: 50, Visible: 0}, 0},
		{"negative", DashboardOffsetParams{Offset: -3, Total: 50, Visible: 10}, 0},
		{"in range", DashboardOffsetParams{Offset: 7, Total: 50, Visible: 10}, 7},
		{"past the end", DashboardOffsetParams{Offset: 999, Total: 50, Visible: 10}, 40},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DashboardClampOffset(tc.params); got != tc.want {
				t.Errorf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDashboardScrollOffsetToleratesAnOutOfRangeCursor(t *testing.T) {
	got := DashboardScrollOffset(DashboardScrollParams{Cursor: 99, Total: 10, Visible: 5, Offset: 99})

	if want := 5; got != want {
		t.Errorf("offset = %d, want %d — a stale cursor must not scroll past the end", got, want)
	}
}

func TestComputeMenuRectHangsUnderItsAnchor(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 10, AnchorY: 5,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.X != 10 || rect.Y != 6 {
		t.Errorf("menu at (%d,%d), want it just under the cell it was opened from", rect.X, rect.Y)
	}
}

func TestComputeMenuRectFlipsAboveTheAnchorAtTheBottom(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 4, AnchorY: 38,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.Y+rect.Height > 40 {
		t.Errorf("menu runs to y=%d, past the screen", rect.Y+rect.Height)
	}
	if rect.Y >= 38 {
		t.Errorf("menu at y=%d, want it flipped above the anchor", rect.Y)
	}
}

func TestComputeMenuRectStaysInsideTheRightEdge(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 115, AnchorY: 2,
		Width: 20, Height: 4,
		ScreenWidth: 120, ScreenHeight: 40,
	})

	if rect.X+rect.Width > 120 {
		t.Errorf("menu spans to x=%d, past the right edge", rect.X+rect.Width)
	}
}

func TestComputeMenuRectSurvivesAScreenSmallerThanTheMenu(t *testing.T) {
	rect := ComputeMenuRect(MenuRectParams{
		AnchorX: 2, AnchorY: 2,
		Width: 40, Height: 10,
		ScreenWidth: 20, ScreenHeight: 6,
	})

	if rect.X < 0 || rect.Y < 0 || rect.X+rect.Width > 20 || rect.Y+rect.Height > 6 {
		t.Errorf("menu = %+v, want it clamped inside a 20x6 screen", rect)
	}
}

func TestSplitRecapFieldSeparatesLabelFromValue(t *testing.T) {
	label, value, ok := SplitRecapField("Parent:      feat → main")
	if !ok {
		t.Fatal("a recap field must split")
	}
	if label != "Parent:      " {
		t.Errorf("label = %q, want the label and the run of spaces after it", label)
	}
	if value != "feat → main" {
		t.Errorf("value = %q", value)
	}
}

// A heading is not a field: it must be left exactly as the flow wrote it.
func TestSplitRecapFieldLeavesHeadingsAndProseWhole(t *testing.T) {
	for _, line := range []string{
		"Will delete:",
		"dev-a is currently rebased onto feat",
		"",
		":no label",
		"Label:no gap",
	} {
		if _, _, ok := SplitRecapField(line); ok {
			t.Errorf("%q must not read as a field", line)
		}
	}
}

func TestActiveWorktree(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", Path: "/repo"},
		{Branch: "feat/ui", Path: "/repo.worktrees/feat-ui"},
		{Branch: "feat/ui-extra", Path: "/repo.worktrees/feat-ui-extra"},
		{Branch: "nested", Path: "/repo.worktrees/feat-ui/nested"},
	}
	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"racine du worktree", "/repo.worktrees/feat-ui", "feat/ui"},
		{"sous-dossier", "/repo.worktrees/feat-ui/internal/tui", "feat/ui"},
		{"préfixe voisin non confondu", "/repo.worktrees/feat-ui-extra", "feat/ui-extra"},
		{"worktree principal", "/repo/internal", "main"},
		{"le plus profond gagne", "/repo.worktrees/feat-ui/nested/pkg", "nested"},
		{"hors de tout worktree", "/ailleurs", ""},
		{"cwd vide", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ActiveWorktree(ActiveWorktreeParams{Cwd: c.cwd, Statuses: statuses})
			if got != c.want {
				t.Errorf("ActiveWorktree(%q) = %q, want %q", c.cwd, got, c.want)
			}
		})
	}
}
