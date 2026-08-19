package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func detailModel(t *testing.T, status domain.WorktreeStatus) Model {
	t.Helper()
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, worktreesMsg{statuses: []domain.WorktreeStatus{status}, parents: map[string]string{}})
}

func lineContaining(t *testing.T, lines []string, needle string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no line contains %q", needle)
	return ""
}

// detailLinksRow finds the row index carrying the LINKS section's title, the
// signal state 3 (§8) must not move when the placeholder it reserves is
// replaced by the sections that actually land.
func detailLinksRow(t *testing.T, view string) int {
	t.Helper()
	lines := strings.Split(view, "\n")
	for index, line := range lines {
		if strings.Contains(line, domain.DetailSectionLinks) {
			return index
		}
	}
	t.Fatal("the rendered view has no LINKS section")
	return -1
}

// The list and detail panels sit side by side, so a raw model.View() line
// mixes both panels' content on one terminal row — asserting on the detail
// panel's own lines is what actually isolates its title row.
func TestDetailHeaderCarriesIdentityNotState(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	lines := model.detailBody(model.layout())

	titleLine := lineContaining(t, lines, "feat/x")
	if strings.Contains(titleLine, "dirty") {
		t.Error("the state pill belongs to the vital strip, not the title line")
	}
}

func TestDetailOmitsEmptyFields(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	view := model.View()

	for _, dead := range []string{"PR       none", "Parent   —", "up to date"} {
		if strings.Contains(view, dead) {
			t.Errorf("the panel must not recite %q anymore", dead)
		}
	}
}

func TestVitalStripWrapsChipByChip(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{
		Branch: "feat/x", Path: "/wt/x", IsDirty: true, CommitsAhead: 3,
		OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
	})
	model = update(model, tea.WindowSizeMsg{Width: narrowWide, Height: testHeight})

	if strings.Contains(model.View(), "↓…") {
		t.Error("a chip is never cut mid-way: that would be a lie, not a truncation")
	}
}

func TestFirstLoadReservesHeightInsteadOfJumping(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model.detailLoading = "feat/x"
	model.detailSince = time.Now().Add(-time.Second)

	before := detailLinksRow(t, model.View())
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:  "feat/x",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
	}})
	after := detailLinksRow(t, model.View())

	if before != after {
		t.Errorf("LINKS moved from row %d to row %d — the placeholder must reserve its height", before, after)
	}
	if !strings.Contains(model.View(), "abc1234") {
		t.Error("the landed data must replace the placeholder")
	}
}

func TestLegitimateAbsenceIsNotAFailure(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		EnvDrift: domain.EnvDriftSummary{Configured: false},
	}})

	view := model.View()
	if !strings.Contains(view, domain.DashboardNotConfigured) {
		t.Error("no env file configured is a legitimate absence, not a success to fake")
	}
	if strings.Contains(view, "unavailable") {
		t.Error("a legitimate absence must not read as a failure")
	}
}

func TestFailureSaysWhy(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: errors.New("git error")},
	}})

	view := model.View()
	if !strings.Contains(view, "unavailable") || !strings.Contains(view, "git error") {
		t.Error("a family that failed to read must say why: it never goes silently empty")
	}
}

func TestBlockersRenderAboveSections(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Blockers: []domain.CleanBlocker{{Label: "uncommitted changes"}},
	}})

	view := model.View()
	blockerAt := strings.Index(view, "uncommitted changes")
	linksAt := strings.Index(view, domain.DetailSectionLinks)
	if blockerAt < 0 || linksAt < 0 || blockerAt > linksAt {
		t.Error("blockers read before the sections: they answer why the action is refused")
	}
}

// TestReviewShowsTheGHFailureNotAnEmptyReview and
// TestReviewStaysAbsentWhenGHIsFineAndThereIsNoPR are the two ends of the
// distinction §8 state 4 exists to enforce: an absence caused by a broken gh
// must never render the same as an absence caused by there being nothing.
func TestReviewShowsTheGHFailureNotAnEmptyReview(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, prsMsg{conn: domain.GHConnectionNotInstalled})

	view := model.View()
	if !strings.Contains(view, domain.DetailSectionReview) {
		t.Error("gh unavailable must still render a REVIEW section, naming why, not silence")
	}
	if !strings.Contains(view, "unavailable") {
		t.Error("the REVIEW section must say gh is unavailable, not read as an empty PR line")
	}
}

func TestReviewStaysAbsentWhenGHIsFineAndThereIsNoPR(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, prsMsg{conn: domain.GHConnectionOK})

	if strings.Contains(model.View(), domain.DetailSectionReview) {
		t.Error("gh fine and no PR is a legitimate absence: no REVIEW section at all")
	}
}
