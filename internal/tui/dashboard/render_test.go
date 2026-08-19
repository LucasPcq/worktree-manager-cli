package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These three paths — repoName from ProjectDir, activeBranch from Cwd via
// rules.ActiveWorktree, fetchedAt from a reload — are exactly what this task
// was supposed to repair. Every other header test in this file hand-assigns
// the fields to pin the rendering; this one goes through New and a real
// worktreesMsg instead, so deleting the wiring in applyWorktrees (rather than
// just the rendering) fails it too.
func TestHeaderIsWiredThroughTheRealPath(t *testing.T) {
	const (
		projectDir = "/repo/worktree-manager-cli"
		mainPath   = "/repo/worktree-manager-cli"
		activePath = "/repo/worktree-manager-cli.worktrees/feat-a"
	)

	model := New(RunParams{ProjectDir: projectDir, Cwd: activePath})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	wtStatuses := []domain.WorktreeStatus{
		{Branch: "main", Path: mainPath},
		{Branch: "feat/a", Path: activePath},
	}
	model = update(model, worktreesMsg{
		statuses:  wtStatuses,
		parents:   map[string]string{},
		fetchedAt: time.Now().Add(-72 * time.Hour),
	})

	first := strings.Split(model.View(), "\n")[0]
	if !strings.Contains(first, "worktree-manager-cli") {
		t.Errorf("context line = %q, want the repo name New() derives from ProjectDir", first)
	}
	if !strings.Contains(first, "feat/a") {
		t.Errorf("context line = %q, want the worktree Cwd falls under, via rules.ActiveWorktree in applyWorktrees", first)
	}
	if !strings.Contains(first, "3 d ago") {
		t.Errorf("context line = %q, want the age carried on worktreesMsg.fetchedAt", first)
	}

	// r re-runs loadWorktreesCmd, which re-reads the fetch time; applyWorktrees
	// must take the fresher value rather than keeping the first one forever, or
	// the staleness marker — once shown — would never clear.
	model = update(model, worktreesMsg{
		statuses:  wtStatuses,
		parents:   map[string]string{},
		fetchedAt: time.Now(),
	})
	if strings.Contains(model.View(), "fetched") {
		t.Error("a fresh fetchedAt from a later worktreesMsg must clear the staleness marker")
	}
}

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

	// The two checks above only prove the line fits and the wordmark survives
	// — reversing the variant order (dropping base before the active worktree,
	// say) would pass both just as well. Pin the ORDER itself: a width that
	// fits repo+base+active only without the active segment must show base
	// and drop active, never the reverse.
	order := New(RunParams{})
	t.Cleanup(order.Close)
	order.repoName, order.baseBranch, order.activeBranch = "demo-repo", "trunk", "wip"

	const (
		widthDropsActiveOnly = 32 // " wtm  demo-repo · base trunk" (28) fits; the +active variant (36) does not.
	)
	line := order.renderContextLine(widthDropsActiveOnly)
	if !strings.Contains(line, "base trunk") {
		t.Errorf("line = %q, want base to survive at this width", line)
	}
	if strings.Contains(line, "wip") {
		t.Errorf("line = %q, want the active worktree dropped before base at this width", line)
	}
}

// TestContextLineGoesEmptyRatherThanOverflowAnUnfittableWidth pins the width
// guard directly: reverting it to "return wordmark" unconditionally would
// make this fail, since the bare wordmark itself does not fit width 4.
func TestContextLineGoesEmptyRatherThanOverflowAnUnfittableWidth(t *testing.T) {
	model := New(RunParams{})
	t.Cleanup(model.Close)

	const tooNarrowForTheWordmark = 4 // the wordmark alone renders 5 columns wide.
	if line := model.renderContextLine(tooNarrowForTheWordmark); line != "" {
		t.Errorf("renderContextLine(%d) = %q, want empty — even the bare wordmark does not fit",
			tooNarrowForTheWordmark, line)
	}
}

func TestHeaderNamesNeverFetchedDistinctlyFromAnAge(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")

	model.fetchedAt = time.Time{}
	if !strings.Contains(model.View(), domain.DashboardNeverFetched) {
		t.Error("un dépôt jamais fetché doit le dire — le cas le plus périmé ne doit pas se taire")
	}

	model.fetchedAt = time.Now().Add(-72 * time.Hour)
	view := model.View()
	if strings.Contains(view, domain.DashboardNeverFetched) {
		t.Error("un fetch daté, même périmé, ne doit pas porter le libellé \"jamais fetché\"")
	}
	if !strings.Contains(view, "fetched") {
		t.Error("un fetch daté et périmé doit toujours annoncer son âge")
	}
}
