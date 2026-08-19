package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The coordinates below are derived from the layout rules, not read back from
// the zone manager: a zone marked over the wrong region is exactly the mistake a
// self-referential assertion would miss.
//
// At 120x40 with the output panel folded:
//
//	y=0            header context line (repo, base branch, active worktree)
//	y=1            header bar (wordmark and tabs)
//	y=2            the rule under the active tab
//	y=3            list/detail top border   (list x=0..47, detail x=48..119)
//	y=4            panel title
//	y=5            the blank line under it
//	y=6+3i         worktree row i           (two lines, then a gap; text at x=2)
//	y=36..38       output panel             (title row y=37)
//	y=39           help bar
const (
	headerBarY   = domain.DashboardHeaderHeight - 2
	titleRowY    = domain.DashboardHeaderHeight + 1
	firstRowY    = titleRowY + 1 + domain.DashboardTitleGap
	rowStride    = domain.DashboardRowHeight + domain.DashboardRowGap
	rowTextX     = 2
	detailX      = 60
	outputTitleY = 37
)

// rowY is the first line of worktree row i.
func rowY(index int) int { return firstRowY + index*rowStride }

func TestRowZonesAreMarkedWhereTheLayoutPutsThem(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(0), rowZone(2))

	first := model.zones.Get(rowZone(0))
	if first.StartX != rowTextX || first.StartY != firstRowY {
		t.Errorf("row 0 starts at (%d,%d), want (%d,%d)", first.StartX, first.StartY, rowTextX, firstRowY)
	}
	if want := model.layout().List.Width - borderWidth - paddingWidth + rowTextX - 1; first.EndX != want {
		t.Errorf("row 0 ends at x=%d, want %d — it must not reach into the detail panel", first.EndX, want)
	}

	third := model.zones.Get(rowZone(2))
	if third.StartY != rowY(2) {
		t.Errorf("row 2 sits at y=%d, want %d", third.StartY, rowY(2))
	}
	if height := third.EndY - third.StartY + 1; height != domain.DashboardRowHeight {
		t.Errorf("row 2 is %d lines tall, want %d", height, domain.DashboardRowHeight)
	}
}

func TestClickingARowSelectsThatWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	model = update(model, click(rowTextX+3, rowY(2)))

	if model.cursor != 2 {
		t.Fatalf("cursor = %d, want the clicked row 2", model.cursor)
	}
	selected, _ := model.selected()
	if selected.Branch != "c" {
		t.Errorf("selected %q, want the worktree the click landed on", selected.Branch)
	}
}

func TestClickingAScrolledRowSelectsTheRightWorktree(t *testing.T) {
	branches := make([]string, 30)
	for i := range branches {
		branches[i] = string(rune('a' + i))
	}
	// 13 is the shortest height that still fits one full row under the 3-line
	// header, the panel chrome and the folded output panel.
	model := newTestModel(t, testWidth, 13, branches...)
	model = update(model, key("G"))
	renderAndWait(t, model, rowZone(model.offset))

	model = update(model, click(rowTextX, firstRowY))

	if model.cursor != model.offset {
		t.Fatalf("cursor = %d, want the first visible row (%d) — zones must key on the absolute index",
			model.cursor, model.offset)
	}
}

func TestClickingOutsideTheListLeavesTheSelectionAlone(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(0), zoneDetail)

	for _, where := range []tea.MouseMsg{
		click(detailX, firstRowY), // the detail panel, beside row 0
		click(rowTextX, rowY(3)),  // below the last row
		click(rowTextX, 0),        // the header bar
	} {
		if got := update(model, where).cursor; got != 0 {
			t.Errorf("click at (%d,%d) moved the cursor to %d", where.X, where.Y, got)
		}
	}
}

func TestClickingTheOutputHeaderFoldsAndUnfolds(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	renderAndWait(t, model, zoneOutputToggle)

	model = update(model, click(10, outputTitleY))
	if !model.outputExpanded {
		t.Fatal("clicking the output header must unfold the panel")
	}

	// Unfolded, the panel grew upwards: its title row moved with it.
	expandedTitleY := model.layout().Output.Y + 1
	renderAndWaitAt(t, model, zoneOutputToggle, expandedTitleY)
	model = update(model, click(10, expandedTitleY))
	if model.outputExpanded {
		t.Fatal("clicking the header again must fold it back")
	}
}

func TestClickingATabActivatesIt(t *testing.T) {
	defer func(saved []string) { tabs = saved }(tabs)
	tabs = []string{domain.DashboardTabWorktrees, "Configuration"}

	model := newTestModel(t, testWidth, testHeight, "a")
	renderAndWait(t, model, tabZone(0), tabZone(1))

	second := model.zones.Get(tabZone(1))
	if second.StartY != headerBarY {
		t.Fatalf("tab 1 sits at y=%d, want the tab bar row", second.StartY)
	}
	if first := model.zones.Get(tabZone(0)); second.StartX <= first.EndX {
		t.Fatalf("tab 1 starts at x=%d but tab 0 ends at x=%d — the tabs overlap", second.StartX, first.EndX)
	}

	model = update(model, click(second.StartX+1, headerBarY))
	if model.tab != 1 {
		t.Errorf("tab = %d after clicking the second tab, want 1", model.tab)
	}
}

func TestWheelOverTheListMovesTheSelection(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, zoneList)

	model = update(model, wheel(rowTextX, firstRowY, tea.MouseButtonWheelDown))
	if model.cursor != 1 {
		t.Fatalf("cursor = %d after a wheel-down over the list, want 1", model.cursor)
	}

	model = update(model, wheel(rowTextX, firstRowY, tea.MouseButtonWheelUp))
	if model.cursor != 0 {
		t.Errorf("cursor = %d after a wheel-up, want 0", model.cursor)
	}
}

func TestWheelOverTheOutputPanelScrollsItInsteadOfTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key(domain.KeyToggleOutput))
	for i := 0; i < 30; i++ {
		model = update(model, OutputLineMsg{Text: "line"})
	}
	renderAndWait(t, model, zoneOutput)

	insideOutput := model.layout().Output.Y + 2
	before := model.outputOffset

	model = update(model, wheel(10, insideOutput, tea.MouseButtonWheelUp))

	if model.outputOffset != before-1 {
		t.Errorf("output offset = %d, want %d", model.outputOffset, before-1)
	}
	if model.cursor != 0 {
		t.Error("a wheel over the output panel must not move the worktree selection")
	}
}

func TestWheelOverAFoldedOutputPanelIsInert(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneOutput)

	model = update(model, wheel(10, outputTitleY, tea.MouseButtonWheelDown))

	if model.cursor != 0 || model.outputOffset != 0 {
		t.Errorf("folded panel: cursor=%d offset=%d, want both untouched", model.cursor, model.outputOffset)
	}
}

func TestNarrowClickOnARowOpensTheDetail(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "a", "b")
	renderAndWait(t, model, rowZone(1))

	model = update(model, click(rowTextX, rowY(1)))

	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want the clicked row", model.cursor)
	}
	if !model.detailOpen {
		t.Error("a narrow terminal must open the detail on a row click, mirroring enter")
	}
}

// The right button is no longer among them: it opens the context menu, which
// TestRightClickSelectsTheRowAndOpensItsMenu covers.
func TestNonLeftMouseEventsAreIgnored(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	ignored := []tea.MouseMsg{
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionRelease, Button: tea.MouseButtonRight},
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionMotion, Button: tea.MouseButtonNone},
	}
	for _, msg := range ignored {
		if got := update(model, msg).cursor; got != 0 {
			t.Errorf("%v moved the cursor to %d", msg, got)
		}
	}
}

func TestHelpOverlaySwallowsMouseEvents(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))
	model = update(model, key(domain.KeyHelp))

	model = update(model, click(rowTextX, rowY(2)))

	if model.cursor != 0 {
		t.Error("clicks must not reach the list through the help overlay")
	}
}

// Both global actions live in the header bar: they act on the repository, not on
// the panel below them, and the Tree tab has no list header to hang them from.
func TestTheHeaderCarriesBothGlobalActions(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneAdd, zoneActions)

	add, actions := model.zones.Get(zoneAdd), model.zones.Get(zoneActions)

	if add.StartY != headerBarY || actions.StartY != headerBarY {
		t.Errorf("buttons on rows %d and %d, want both on the header bar", add.StartY, actions.StartY)
	}
	if add.EndX >= actions.StartX {
		t.Errorf("the call to action ends at %d and Actions starts at %d, want them in that order", add.EndX, actions.StartX)
	}
}

// zonePoint is the first cell of a zone, the one a click lands on.
func zonePoint(model Model, id string) (x, y int) {
	zone := model.zones.Get(id)
	return zone.StartX, zone.StartY
}

func TestClickingTheAddButtonStartsTheSameRunAsTheKey(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneAdd)
	x, y := zonePoint(model, zoneAdd)

	clicked, cmd := updateCmd(model, click(x, y))

	if cmd == nil || len(clicked.ops.running) != 1 {
		t.Fatalf("clicking the add button started %+v, want one create run", clicked.ops.running)
	}
	if clicked.cursor != 0 {
		t.Error("the add button must not double as a row click")
	}
}

func TestClickingTheActionsButtonOpensTheGlobalMenu(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneActions)
	x, y := zonePoint(model, zoneActions)

	opened := update(model, click(x, y))

	if !opened.menuOpen {
		t.Fatal("the Actions button must open its menu")
	}
	if opened.menuKind != menuForGlobal {
		t.Error("the header button opens the global menu, not the row's")
	}
}
