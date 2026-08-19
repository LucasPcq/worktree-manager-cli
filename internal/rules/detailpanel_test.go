package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sectionKeys(sections []domain.DetailSection) []string {
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		keys = append(keys, section.Key)
	}
	return keys
}

func TestDetailSectionsOmitsWhatHasNothingToSay(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Detail: domain.WorktreeDetail{Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}}},
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("REVIEW ne doit pas apparaître sans PR")
		}
		if key == domain.DetailSectionChanges {
			t.Error("CHANGES ne doit pas apparaître sur un worktree propre")
		}
	}
}

func TestDetailSectionsKeepsFixedOrder(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		PR:     &domain.PRInfo{Number: 67, Title: "feat: x", State: "OPEN"},
		Detail: domain.WorktreeDetail{
			Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
			Changes: domain.WorkingChanges{Modified: 2, Files: []domain.PorcelainEntry{{Status: " M", Path: "a.go"}}},
		},
	})

	want := []string{
		domain.DetailSectionReview,
		domain.DetailSectionChanges,
		domain.DetailSectionActivity,
		domain.DetailSectionLinks,
	}
	got := sectionKeys(sections)
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFitSectionsDropsFromTheBottom(t *testing.T) {
	sections := []domain.DetailSection{
		{Key: domain.DetailSectionReview, Lines: []string{"a", "b"}},
		{Key: domain.DetailSectionChanges, Lines: []string{"c", "d"}},
		{Key: domain.DetailSectionActivity, Lines: []string{"e", "f"}},
		{Key: domain.DetailSectionLinks, Lines: []string{"g", "h"}},
	}

	got := sectionKeys(FitSections(FitSectionsParams{Sections: sections, Height: 8}))
	want := []string{domain.DetailSectionReview, domain.DetailSectionChanges}
	if len(got) != len(want) {
		t.Fatalf("sections retenues = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVitalChipsStateFirstAndOnlyStateColored(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	chips := VitalChips(VitalChipsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", IsDirty: true, CommitsAhead: 3,
			OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
		},
		LastCommitAt: now.Add(-3 * time.Hour),
		Now:          now,
	})

	if len(chips) == 0 {
		t.Fatal("aucun chip")
	}
	if !chips[0].State {
		t.Error("le premier chip doit être l'état : c'est la lecture la plus rapide")
	}
	for i, chip := range chips[1:] {
		if chip.State {
			t.Errorf("chip[%d] est marqué State — l'état est le seul chip coloré", i+1)
		}
	}
}

func TestVitalChipsNeverMentionsCreated(t *testing.T) {
	now := time.Now()
	chips := VitalChips(VitalChipsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", CreatedAt: now.Add(-48 * time.Hour)},
		LastCommitAt: now.Add(-time.Hour),
		Now:          now,
	})
	for _, chip := range chips {
		if chip.Text == "" {
			t.Error("un chip vide ne doit pas être émis")
		}
		if len(chip.Text) >= 7 && chip.Text[:7] == "created" {
			t.Error("created appartient à LINKS, pas à la bande vitale")
		}
	}
}
