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

func TestActiveRowIsMarked(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.activeBranch = "feat/a"

	row := lineContaining(t, strings.Split(model.View(), "\n"), "feat/a")
	if !strings.Contains(row, domain.DashboardActiveGlyph) {
		t.Error("la ligne du worktree actif doit porter son marqueur")
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
