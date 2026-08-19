package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSelectionSchedulesDetailLoad(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	if model.detailLoading != "feat/a" {
		t.Errorf("detailLoading = %q, want feat/a", model.detailLoading)
	}
}

func TestCachedDetailSurvivesReload(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{
		Branch:  "main",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: seed"}},
	}})

	model = update(model, pollMsg{})

	got, ok := model.details["main"]
	if !ok {
		t.Fatal("le poll ne doit pas vider le cache : le panneau afficherait du vide")
	}
	if len(got.Commits) != 1 {
		t.Errorf("Commits = %d, want 1", len(got.Commits))
	}
}

// TestParentPathIgnoresDetachedWorktree pins the statusFor("") guard: a
// detached worktree can carry an empty Branch through service/worktree.List,
// and a branch with no recorded parent must not resolve to that worktree's
// path. Getting this wrong feeds the wrong ParentPath into ComputeEnvDiff,
// silently — wrong .env drift counters, with no error anywhere.
func TestParentPathIgnoresDetachedWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "feat/a")
	model.statuses = append(model.statuses, domain.WorktreeStatus{Branch: "", Path: "/tmp/detached"})
	model.parents = map[string]string{"feat/a": ""}

	params := model.detailParams("feat/a")
	if params.ParentPath != "" {
		t.Errorf("ParentPath = %q, want empty: a parentless branch must not borrow a detached worktree's path", params.ParentPath)
	}
}

// TestChildrenAreSorted pins that childrenOf never returns map iteration
// order: the selected branch's detail reloads regularly, and an unsorted
// LINKS "Children" line would permute between refreshes — content moving
// under state 2 (§8), which must hold pixel-still.
func TestChildrenAreSorted(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.parents = map[string]string{
		"feat/z": "main",
		"feat/a": "main",
		"feat/m": "main",
	}

	got := model.childrenOf("main")
	want := []string{"feat/a", "feat/m", "feat/z"}
	if len(got) != len(want) {
		t.Fatalf("childrenOf = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("childrenOf = %v, want %v", got, want)
			break
		}
	}
}

func TestStaleDetailIsMarkedNotErrored(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})
	model.detailLoading = "main"
	model.detailSince = time.Now().Add(-time.Second)

	view := model.View()
	if !strings.Contains(view, domain.DashboardRefreshing) {
		t.Error("un détail en cours de rechargement doit porter son marqueur")
	}
}

func TestFreshDetailShowsNoMarker(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	// Asserted directly, not just through the view: a detailLoading that
	// applyDetail forgot to clear would still pass the view-only check below
	// for up to DashboardSpinnerGrace, making this test vacuous.
	if model.detailLoading != "" {
		t.Fatalf("detailLoading = %q, want empty: a landed load must clear it", model.detailLoading)
	}
	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("l'état frais ne se signale pas")
	}
}

// TestSupersededTickIsDropped pins the one line that makes scheduleDetail a
// debounce rather than a plain delay: a tick landing for a branch that is no
// longer selected must start no load at all.
func TestSupersededTickIsDropped(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	model = update(model, key("j"))
	if model.detailLoading != "feat/b" {
		t.Fatalf("detailLoading = %q, want feat/b", model.detailLoading)
	}

	_, tickCmd := updateCmd(model, detailTickMsg{branch: "feat/a"})
	if tickCmd != nil {
		t.Error("un tick pour une branche qui n'est plus sélectionnée ne doit lancer aucun chargement")
	}
}

// TestPollSkipsDetailReloadWhenPanelHidden pins §7's "jamais dans le poll"
// rule extended to visibility: a poll must do no detail work at all when
// nothing is on screen to keep fresh.
func TestPollSkipsDetailReloadWhenPanelHidden(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "main")
	if model.layout().DetailVisible {
		t.Fatal("setup: narrow mode without detailOpen must hide the detail panel")
	}
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	model = update(model, pollMsg{})
	if model.detailLoading != "" {
		t.Errorf("detailLoading = %q, want empty: the poll must not reload a hidden detail panel", model.detailLoading)
	}
}

// TestDebounceIsNotCountedAgainstTheGraceDelay pins that detailSince is set
// when the load actually starts (fireDetailTick), not when it is merely
// scheduled: otherwise the 150 ms debounce eats most of the 200 ms grace
// budget, and an ordinary git log flashes the spinner it was meant to avoid.
func TestDebounceIsNotCountedAgainstTheGraceDelay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	if model.detailIsStale() {
		t.Fatal("detailIsStale must be false the instant a load is only scheduled, not started")
	}

	model = update(model, detailTickMsg{branch: "feat/a"})
	if model.detailSince.IsZero() {
		t.Fatal("detailSince must be set once the debounce lands and the load actually starts")
	}
	if model.detailIsStale() {
		t.Error("the grace delay must restart at the actual load, not be pre-consumed by the debounce")
	}
}

func TestMarkerWaitsForTheGraceDelay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.detailLoading = "main"
	model.detailSince = time.Now()

	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("sous le délai d'apparition, aucun marqueur : ce serait du flash déguisé en feedback")
	}
}
