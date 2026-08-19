package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestHeaderCarriesRepoAndActiveWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.repoName = "worktree-manager-cli"
	model.baseBranch = "main"
	model.activeBranch = "feat/a"

	first := strings.Split(model.View(), "\n")[0]
	for _, want := range []string{"wtm", "worktree-manager-cli", "main", "feat/a"} {
		if !strings.Contains(first, want) {
			t.Errorf("ligne 1 du header = %q, doit contenir %q", first, want)
		}
	}
}

func TestHeaderAnnouncesStaleFetchOnlyPastTheThreshold(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")

	model.fetchedAt = time.Now().Add(-time.Hour)
	if strings.Contains(model.View(), "fetched") {
		t.Error("un fetch récent ne s'annonce pas : un marqueur permanent ne signale plus rien")
	}

	model.fetchedAt = time.Now().Add(-72 * time.Hour)
	if !strings.Contains(model.View(), "fetched") {
		t.Error("au-delà du seuil, la vue doit déclarer que ses refs origin sont périmées")
	}
}

func TestHeaderDropsSegmentsRightToLeftWhenNarrow(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "main")
	model.repoName = "un-nom-de-depot-vraiment-tres-long"
	model.baseBranch = "main"
	model.activeBranch = "feat/une-branche-au-nom-interminable"
	model.fetchedAt = time.Now().Add(-72 * time.Hour)

	first := strings.Split(model.View(), "\n")[0]
	if lipgloss.Width(first) > narrowWide {
		t.Errorf("ligne 1 large de %d, max %d", lipgloss.Width(first), narrowWide)
	}
	if !strings.Contains(first, domain.DashboardWordmark) {
		t.Error("le wordmark est le dernier segment à tomber")
	}
}
