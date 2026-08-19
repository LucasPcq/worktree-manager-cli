package dashboard

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// lockOperation puts a background create in flight, lets it name its target the
// way the prompter does once the wizard is answered, and posts the stage a
// running flow's Presenter would send from Stage/HookPhase.
func lockOperation(t *testing.T, model Model, branch, stage string) Model {
	t.Helper()
	model, cmd := updateCmd(model, key(domain.KeyNew))
	if cmd == nil {
		t.Fatal("the create run did not start")
	}
	id := model.ops.running[0].id
	model, _ = model.applyFlow(opTargetMsg{id: id, target: branch})
	model, _ = model.applyFlow(opStageMsg{id: id, stage: stage})
	if _, held := model.ops.holding(branch); !held {
		t.Fatalf("the run does not hold %q", branch)
	}
	return model
}

// TestActiveRowIsMarked pins that the active worktree is named in the
// row's meta line, in the same tag vocabulary as "from main" or "PR #67" —
// not a bare glyph glued to the branch name, which communicated nothing on
// its own and appeared nowhere else in the row.
func TestActiveRowIsMarked(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.activeBranch = "feat/a"

	// listBody isolates the list panel from the header, which carries its own
	// "● <branch>" segment (untouched here) — a raw model.View() line would
	// find that one first and prove nothing about the row itself. Each entry
	// still embeds its own two physical lines joined by "\n", so split again
	// before searching one line at a time.
	lines := strings.Split(strings.Join(model.listBody(model.layout()), "\n"), "\n")
	nameRow := lineContaining(t, lines, "feat/a")
	if strings.Contains(nameRow, domain.DashboardActiveGlyph) {
		t.Error("the branch name must not carry a bare glyph — the marker moved to the meta line as a text tag")
	}

	metaRow := lineContaining(t, lines, domain.DetailYouAreHere)
	if !strings.Contains(metaRow, domain.DetailYouAreHere) {
		t.Error("la ligne meta du worktree actif doit porter le tag \"you are here\"")
	}
}

func TestLockedRowShowsProgressNotState(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = lockOperation(t, model, "feat/a", "running on_create hook…")

	// Read the list panel's own body, not the full joined view: at this width
	// the detail panel sits on the same terminal rows, and "main" (still the
	// cursor) legitimately shows "clean" there — nothing to do with feat/a's row.
	body := strings.Join(model.listBody(model.layout()), "\n")
	nameLine := lineContaining(t, strings.Split(body, "\n"), "feat/a")
	if strings.Contains(nameLine, "clean") || strings.Contains(nameLine, "dirty") {
		t.Error("la pastille d'état cède sa place au spinner pendant l'opération")
	}
	if !strings.Contains(body, "running on_create hook…") {
		t.Error("une ligne verrouillée montre l'étape en cours")
	}
}

// TestEmptySelectionAndOutputNameTheNextAction covers the two empty states
// that are genuinely reachable and persistent: no worktree selected, and no
// operation has produced output yet. DashboardEmptyList is deliberately not
// asserted here — the list can only be empty when the listing itself did not
// go as expected (a valid repository always has its main worktree), so it
// keeps neutral wording rather than naming an action.
func TestEmptySelectionAndOutputNameTheNextAction(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model.outputExpanded = true

	if !strings.Contains(strings.Join(model.detailBody(model.layout()), "\n"), domain.DashboardEmptySelection) {
		t.Error("aucune sélection : le panneau détail doit nommer l'action suivante")
	}
	if !strings.Contains(strings.Join(model.outputBody(model.layout(), testWidth), "\n"), domain.DashboardEmptyOutput) {
		t.Error("aucune sortie : le panneau output doit nommer l'action suivante")
	}
}

// TestEmptyListStaysNeutral pins the deliberate exception: an empty listing
// only happens when the listing itself did not go as expected (a valid
// repository always has its main worktree), so the placeholder must not
// offer confident advice ("press n…") about a situation it cannot explain.
func TestEmptyListStaysNeutral(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)

	body := strings.Join(model.listBody(model.layout()), "\n")
	if !strings.Contains(body, domain.DashboardEmptyList) {
		t.Fatal("l'état vide de la liste est absent")
	}
	if strings.Contains(body, "press n") {
		t.Error("une liste vide ne doit pas conseiller une action : on ignore pourquoi elle est vide")
	}
}
