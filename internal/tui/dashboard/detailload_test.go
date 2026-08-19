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

	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("l'état frais ne se signale pas")
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
