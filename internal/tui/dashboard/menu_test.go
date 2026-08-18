package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestTheMenuEntriesHangUnderTheirOwnRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0), rowZone(2))

	entry := model.zones.Get(menuZone(0))
	if want := firstRowY + 2; entry.StartY != want {
		t.Errorf("the first entry sits at y=%d, want %d — right under the row it acts on", entry.StartY, want)
	}
	if row := model.zones.Get(rowZone(2)); row.StartY != firstRowY+3 {
		t.Errorf("row 2 sits at y=%d, want %d — the dropdown pushes it down", row.StartY, firstRowY+3)
	}
	if entry.EndX > model.layout().List.Width {
		t.Errorf("the entry reaches x=%d, past the list panel", entry.EndX)
	}
}

func TestClickingTheDeleteEntryStartsTheRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))

	clicked, cmd := updateCmd(model, click(rowTextX+4, firstRowY+2))

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

// The menu pushes the rows under it down, so the row drawn one line below the
// entry is the second one, not the third.
func TestClickingOffTheMenuClosesItAndSelectsWhatWasDrawnThere(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0), rowZone(1))

	if row := model.zones.Get(rowZone(1)); row.StartY != firstRowY+2 {
		t.Fatalf("row 1 sits at y=%d, want %d under the open menu", row.StartY, firstRowY+2)
	}

	model = update(model, click(rowTextX, firstRowY+2))

	if model.menuOpen {
		t.Fatal("a click elsewhere closes the menu")
	}
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want the row the click landed on", model.cursor)
	}
}
