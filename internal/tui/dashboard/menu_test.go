package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

func rightClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
}

func TestRightClickSelectsTheRowAndOpensItsMenu(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	model = update(model, rightClick(rowTextX+2, firstRowY+2))

	if model.cursor != 2 {
		t.Fatalf("cursor = %d, want the row the menu was opened on", model.cursor)
	}
	if !model.menuOpen {
		t.Fatal("a right click on a row opens its context menu")
	}
}

func TestRightClickOutsideTheListOpensNothing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, rowZone(0))

	model = update(model, rightClick(detailX, firstRowY))

	if model.menuOpen {
		t.Error("the menu belongs to a worktree row; nowhere else opens it")
	}
}

// The keyboard path is not a convenience: terminals that turn the right button
// into a paste never deliver it, and the dashboard has to stay fully usable.
func TestTheMenuKeyOpensTheSameMenuAsTheRightClick(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))

	model = update(model, key(domain.KeyMenu))

	if !model.menuOpen {
		t.Fatal("m must open the context menu on the selected row")
	}
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want the selection left where it was", model.cursor)
	}
	if len(model.menuItems()) == 0 {
		t.Fatal("the menu must offer something")
	}

	if model = update(model, key(domain.KeyMenu)); model.menuOpen {
		t.Error("m must close the menu it opened")
	}
}

func TestTheMenuNeedsAWorktreeToActOn(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)

	model = update(model, key(domain.KeyMenu))

	if model.menuOpen {
		t.Error("an empty list has no row to open a menu on")
	}
}

func TestEscClosesTheMenuAndOtherKeysFallThrough(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyMenu))

	model = update(model, namedKey(tea.KeyEsc))
	if model.menuOpen {
		t.Fatal("esc closes the menu")
	}

	model = update(model, key(domain.KeyMenu))
	model = update(model, key("G"))

	if model.menuOpen {
		t.Fatal("a key the menu does not use closes it rather than trapping the keyboard")
	}
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want the key to have reached the list", model.cursor)
	}
}

// menuEntryPoint is where entry i is drawn, derived from the placement rule and
// the box's own composition (border row, then the title naming the worktree).
func menuEntryPoint(t *testing.T, model Model, index int) (x, y int) {
	t.Helper()
	box, rect := model.menuBox()
	if box == "" {
		t.Fatal("the menu has nothing to draw")
	}
	return rect.X + menuBorder + menuPadding, rect.Y + menuBorder + menuTitleRows + index
}

const (
	menuBorder    = 1
	menuPadding   = 1
	menuTitleRows = 1
)

func TestTheMenuFloatsUnderTheCellItWasOpenedFrom(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))

	_, rect := model.menuBox()
	if rect.Y != model.menuAnchor.Y+1 {
		t.Errorf("the menu sits at y=%d, want it hanging just under its row (%d)", rect.Y, model.menuAnchor.Y+1)
	}
	if want := firstRowY + 1; model.menuAnchor.Y != want {
		t.Errorf("the keyboard anchored the menu at y=%d, want the selected row %d", model.menuAnchor.Y, want)
	}

	entry := model.zones.Get(menuZone(0))
	wantX, wantY := menuEntryPoint(t, model, 0)
	if entry.StartY != wantY || entry.StartX != wantX {
		t.Errorf("entry 0 starts at (%d,%d), want (%d,%d) — inside the box the rule placed",
			entry.StartX, entry.StartY, wantX, wantY)
	}
}

func TestTheMenuFlipsAboveTheAnchorRatherThanRunningOffTheBottom(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, rightClick(rowTextX, testHeight-1))

	box, rect := model.menuBox()
	if rect.Y+lipgloss.Height(box) > testHeight {
		t.Errorf("the menu runs to y=%d, past the last row %d", rect.Y+lipgloss.Height(box), testHeight)
	}
	if rect.Y >= testHeight-1 {
		t.Errorf("the menu sits at y=%d, want it above an anchor on the last row", rect.Y)
	}
}

func TestClickingAnEntryStartsTheRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))
	x, y := menuEntryPoint(t, model, 0)

	clicked, cmd := updateCmd(model, click(x, y))

	if cmd == nil {
		t.Fatal("clicking Delete must start the removal")
	}
	if clicked.menuOpen {
		t.Error("activating an entry closes the menu")
	}
	if len(clicked.ops.running) != 1 || clicked.ops.running[0].kind != domain.OpKindClean {
		t.Fatalf("running = %+v, want the clean run recorded", clicked.ops.running)
	}
}

func TestEnterOnTheDeleteEntryStartsTheSameRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyMenu))

	model, cmd := updateCmd(model, namedKey(tea.KeyEnter))

	if cmd == nil || len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want enter to start the same run the click does", model.ops.running)
	}
}

// The frame under an open menu carries no zone at all, so a click beside the menu
// dismisses it and nothing else — which is what a context menu does everywhere.
func TestClickingOffTheMenuOnlyClosesIt(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))

	// Right on a row of the frame, whose zone the last unobstructed frame left
	// behind: the menu still swallows it.
	model = update(model, click(rowTextX, firstRowY+2))

	if model.menuOpen {
		t.Fatal("a click elsewhere closes the menu")
	}
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want the dismissing click to have selected nothing", model.cursor)
	}
}

func TestTheFrameIsNotClickableUnderAnOverlay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2), zoneAdd)

	model = update(model, key(domain.KeyMenu))
	model.View()

	// Marking the frame under an overlay would mean cutting through its markers
	// when the box is pasted over it, and losing the zones they carried.
	if _, ok := model.marks().(noMarks); !ok {
		t.Errorf("marks() = %T while the menu is open, want the frame left unmarked", model.marks())
	}
}
