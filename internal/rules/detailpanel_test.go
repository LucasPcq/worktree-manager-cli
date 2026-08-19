package rules

import (
	"strings"
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

func TestReviewShowsUnavailableReasonWhenPRDataFailedToLoad(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:        domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PRUnavailable: "GitHub CLI not found",
	})

	var review *domain.DetailSection
	for i := range sections {
		if sections[i].Key == domain.DetailSectionReview {
			review = &sections[i]
		}
	}
	if review == nil {
		t.Fatal("un gh cassé doit produire une section REVIEW, pas son absence silencieuse")
	}
	if len(review.Lines) != 1 || !strings.Contains(review.Lines[0], "GitHub CLI not found") {
		t.Errorf("REVIEW = %v, want the unavailable reason", review.Lines)
	}
}

func TestReviewStaysAbsentWithNoPRAndNoFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("pas de PR et gh disponible : REVIEW doit rester absente, exactement comme aujourd'hui")
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

	// DetailSectionChrome = 3 (spec §6: title + blank above + blank below), so
	// two 2-line sections cost 2*(3+2) = 10, not 8 as before that correction.
	got := sectionKeys(FitSections(FitSectionsParams{Sections: sections, Height: 10}))
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

func TestChangesSectionSummaryIsOnTitleRowNotALine(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height: 24,
		Detail: domain.WorktreeDetail{
			Changes: domain.WorkingChanges{
				Modified:  2,
				Untracked: 1,
				Files: []domain.PorcelainEntry{
					{Status: " M", Path: "a.go"},
					{Status: " M", Path: "b.go"},
					{Status: "??", Path: "c.go"},
				},
			},
		},
	})

	var changes domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionChanges {
			changes = section
		}
	}
	if changes.Key == "" {
		t.Fatal("CHANGES absente alors que le worktree est sale")
	}
	if !strings.Contains(changes.TitleRight, "2 modified") || !strings.Contains(changes.TitleRight, "1 untracked") {
		t.Errorf("TitleRight = %q, doit porter le résumé des comptes", changes.TitleRight)
	}
	if len(changes.Lines) == 0 || !strings.Contains(changes.Lines[0], "a.go") {
		t.Errorf("Lines[0] = %q, doit être le premier fichier — le résumé n'est plus une ligne", changes.Lines[0])
	}
	for _, line := range changes.Lines {
		if strings.Contains(line, "modified") {
			t.Errorf("Lines = %v, le résumé ne doit plus apparaître comme une ligne du corps", changes.Lines)
		}
	}
}

func TestSectionsHeightMatchesSpecMockup(t *testing.T) {
	// docs/superpowers/specs/2026-08-19-wtm-ui-identity-design.md §6, lines
	// 130-134: a leading blank, the REVIEW title, a blank under it, then 2 body
	// lines — 5 rows total, not 4. Pins DetailSectionChrome = 3.
	review := domain.DetailSection{
		Key: domain.DetailSectionReview,
		Lines: []string{
			"#67  feat(ui): improve dashboard design  OPEN",
			"checks ✓ 12  ✗ 1  ·  review  changes requested",
		},
	}
	if got := sectionsHeight([]domain.DetailSection{review}); got != 5 {
		t.Errorf("sectionsHeight(REVIEW, 2 lignes de corps) = %d, want 5 (spec §6)", got)
	}
}

func TestListBudgetsLeaveRoomForLinksAtHeight30With18Files(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	files := make([]domain.PorcelainEntry, 18)
	for i := range files {
		files[i] = domain.PorcelainEntry{Status: " M", Path: "file.go"}
	}
	commits := make([]domain.CommitSummary, 5)
	for i := range commits {
		commits[i] = domain.CommitSummary{SHA: "abc1234", Subject: "feat: x", At: now.Add(-time.Hour)}
	}

	// Fully populated REVIEW + LINKS (every LINKS field, all 5 lines) plus 18
	// changed files and 5 commits: with the old DetailFixedRows=10 guess, this
	// stack computes to 34 rows at Height=30 and FitSections drops LINKS,
	// taking Path off screen on a panel that had room for everything.
	params := DetailSectionsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", Path: "/wt/x", IsDirty: true, CreatedAt: now.Add(-48 * time.Hour),
		},
		Parent: "main",
		Height: 30,
		Now:    now,
		PR:     &domain.PRInfo{Number: 67, Title: "feat: x", State: "OPEN"},
		Detail: domain.WorktreeDetail{
			Commits:  commits,
			Changes:  domain.WorkingChanges{Modified: 18, Files: files},
			Children: []string{"chore/deps-bump"},
			EnvDrift: domain.EnvDriftSummary{Configured: true, Missing: 2},
		},
	}

	fit := FitSections(FitSectionsParams{Sections: DetailSections(params), Height: params.Height})

	for _, key := range sectionKeys(fit) {
		if key == domain.DetailSectionLinks {
			return
		}
	}
	t.Errorf("LINKS absente à Height=30 — le panneau avait de la place pour Path (sections retenues: %v)",
		sectionKeys(fit))
}

func TestEnvLineGuardsNilFailureError(t *testing.T) {
	line := envLine(domain.WorktreeDetail{
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: nil},
	})
	if strings.Contains(line, "<nil>") {
		t.Errorf("envLine = %q, une erreur nil ne doit jamais être formatée", line)
	}
}
