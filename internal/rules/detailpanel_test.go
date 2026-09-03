package rules

import (
	"errors"
	"fmt"
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
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Detail:       domain.WorktreeDetail{Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}}},
		DetailLoaded: true,
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
		DetailLoaded:  true,
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
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		DetailLoaded: true,
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("pas de PR et gh disponible : REVIEW doit rester absente, exactement comme aujourd'hui")
		}
	}
}

func TestReviewSectionShowsChecksAndDecision(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR: &domain.PRInfo{
			Number: 67, Title: "feat: x", State: "OPEN",
			Checks:         domain.PRChecks{Passed: 12, Failed: 1},
			ReviewDecision: "CHANGES_REQUESTED",
		},
	})

	var review domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionReview {
			review = section
		}
	}
	if review.Key == "" {
		t.Fatal("REVIEW absente alors qu'une PR existe")
	}

	body := strings.Join(review.Lines, "\n")
	for _, want := range []string{"#67", "feat: x", "12", "changes requested"} {
		if !strings.Contains(body, want) {
			t.Errorf("REVIEW = %q, doit contenir %q", body, want)
		}
	}
}

func TestReviewSectionWithoutChecks(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR:     &domain.PRInfo{Number: 68, Title: "feat: y", State: "OPEN"},
	})
	for _, section := range sections {
		if section.Key != domain.DetailSectionReview {
			continue
		}
		if strings.Contains(strings.Join(section.Lines, "\n"), "checks") {
			t.Error("pas de ligne checks quand aucun check n'a tourné")
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
		DetailLoaded: true,
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

	// DetailSectionChrome = 3 (title + blank above + blank below), so
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
		DetailLoaded: true,
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

func TestSectionsHeightCountsTheChromeAroundEachSection(t *testing.T) {
	// A leading blank, the REVIEW title, a blank under it, then 2 body lines —
	// 5 rows total, not 4. Pins DetailSectionChrome = 3.
	review := domain.DetailSection{
		Key: domain.DetailSectionReview,
		Lines: []string{
			"#67  feat(ui): improve dashboard design  OPEN",
			"checks ✓ 12  ✗ 1  ·  review  changes requested",
		},
	}
	if got := sectionsHeight([]domain.DetailSection{review}); got != 5 {
		t.Errorf("sectionsHeight(REVIEW, 2 lignes de corps) = %d, want 5 (3 de chrome + 2 lignes)", got)
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
		DetailLoaded: true,
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

func TestChangesSectionSaysWhyOnFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height:       24,
		DetailLoaded: true,
		Detail: domain.WorktreeDetail{
			Failures: map[domain.DetailFamily]error{domain.DetailFamilyChanges: errors.New("git status failed")},
		},
	})

	var changes domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionChanges {
			changes = section
		}
	}
	if changes.Key == "" {
		t.Fatal("git status en échec doit produire une section CHANGES qui dit pourquoi, pas son absence")
	}
	if len(changes.Lines) != 1 || !strings.Contains(changes.Lines[0], "git status failed") {
		t.Errorf("CHANGES = %v, want the failure reason", changes.Lines)
	}
}

func TestActivitySectionSaysWhyOnFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Height:       24,
		DetailLoaded: true,
		Detail: domain.WorktreeDetail{
			Failures: map[domain.DetailFamily]error{domain.DetailFamilyCommits: errors.New("git log failed")},
		},
	})

	var activity domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionActivity {
			activity = section
		}
	}
	if activity.Key == "" {
		t.Fatal("git log en échec doit produire une section ACTIVITY qui dit pourquoi, pas son absence")
	}
	if len(activity.Lines) != 1 || !strings.Contains(activity.Lines[0], "git log failed") {
		t.Errorf("ACTIVITY = %v, want the failure reason", activity.Lines)
	}
}

func TestFirstLoadPlaceholdersOrderedLikeRealSections(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height: 40,
		// DetailLoaded left false: state 3, nothing cached yet.
	})

	want := []string{domain.DetailSectionChanges, domain.DetailSectionActivity, domain.DetailSectionLinks}
	got := sectionKeys(sections)
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q — rules/ owns the order, not the renderer", i, got[i], want[i])
		}
	}
	for _, key := range []string{domain.DetailSectionChanges, domain.DetailSectionActivity} {
		for _, section := range sections {
			if section.Key != key {
				continue
			}
			if len(section.Lines) != 1 || section.Lines[0] != domain.DashboardLoadingField {
				t.Errorf("%s placeholder = %v, want a single %q line", key, section.Lines, domain.DashboardLoadingField)
			}
		}
	}
}

func TestFailureLineCarriesTheGlyphNotConfiguredDoesNot(t *testing.T) {
	failure := envLine(domain.WorktreeDetail{
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: errors.New("git error")},
	})
	if !strings.Contains(failure, "⚠") {
		t.Errorf("envLine failure = %q, want the warning glyph", failure)
	}

	absent := envLine(domain.WorktreeDetail{EnvDrift: domain.EnvDriftSummary{Configured: false}})
	if strings.Contains(absent, "⚠") {
		t.Errorf("envLine legitimate absence = %q, must stay glyph-free — that contrast is the point", absent)
	}
}

func TestEnvLineGuardsNilFailureError(t *testing.T) {
	line := envLine(domain.WorktreeDetail{
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: nil},
	})
	if strings.Contains(line, "<nil>") {
		t.Errorf("envLine = %q, une erreur nil ne doit jamais être formatée", line)
	}
}

// TestListBudgetsAreFixedCapsNotStateDriven pins §the reworked rule: each list
// gets a fixed maximum row count regardless of dirty/clean state or how much
// height is actually free — the old split gave the leftover to CHANGES when
// dirty and to ACTIVITY when clean, which read as randomness because the
// reasoning was invisible. A dirty and a clean worktree, given the exact same
// abundant height, must produce the exact same cap for both lists.
func TestListBudgetsAreFixedCapsNotStateDriven(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	files := make([]domain.PorcelainEntry, 20)
	for i := range files {
		files[i] = domain.PorcelainEntry{Status: " M", Path: "file.go"}
	}
	commits := make([]domain.CommitSummary, 20)
	for i := range commits {
		commits[i] = domain.CommitSummary{SHA: "abc1234", Subject: "feat: x", At: now.Add(-time.Hour)}
	}
	detail := domain.WorktreeDetail{
		Commits: commits,
		Changes: domain.WorkingChanges{Modified: 20, Files: files},
	}

	for _, dirty := range []bool{true, false} {
		sections := DetailSections(DetailSectionsParams{
			Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: dirty},
			Height:       100,
			Now:          now,
			Detail:       detail,
			DetailLoaded: true,
		})

		var changes, activity domain.DetailSection
		for _, section := range sections {
			switch section.Key {
			case domain.DetailSectionChanges:
				changes = section
			case domain.DetailSectionActivity:
				activity = section
			}
		}
		if len(changes.Lines) != domain.DashboardDetailChanges {
			t.Errorf("dirty=%v: CHANGES = %d lines, want the fixed cap %d", dirty, len(changes.Lines), domain.DashboardDetailChanges)
		}
		if len(activity.Lines) != domain.DashboardDetailCommits {
			t.Errorf("dirty=%v: ACTIVITY = %d lines, want the fixed cap %d", dirty, len(activity.Lines), domain.DashboardDetailCommits)
		}
	}
}

// TestLinksFieldsAreIndentedLikeTheOtherLists pins that LINKS lines carry the
// same DetailListIndent prefix CHANGES and ACTIVITY already use, so the
// section does not look misaligned against its neighbours.
func TestLinksFieldsAreIndentedLikeTheOtherLists(t *testing.T) {
	section := linksSection(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Parent: "main",
	})
	if len(section.Lines) == 0 {
		t.Fatal("LINKS a besoin d'au moins une ligne pour ce test")
	}
	for _, line := range section.Lines {
		if !strings.HasPrefix(line, domain.DetailListIndent) {
			t.Errorf("ligne LINKS = %q, want le préfixe %q comme CHANGES/ACTIVITY", line, domain.DetailListIndent)
		}
	}
}

// TestReviewLinesAreIndentedLikeTheOtherLists pins that REVIEW's PR header
// line and its checks/review-decision line carry the same DetailListIndent
// prefix as CHANGES/ACTIVITY/LINKS, so no section looks misaligned against
// its neighbours.
func TestReviewLinesAreIndentedLikeTheOtherLists(t *testing.T) {
	section := reviewSection(reviewSectionParams{
		PR: &domain.PRInfo{
			Number: 67, Title: "feat: x", State: "OPEN",
			Checks:         domain.PRChecks{Passed: 12, Failed: 1},
			ReviewDecision: domain.GHReviewDecisionApproved,
		},
	})
	if len(section.Lines) != 2 {
		t.Fatalf("REVIEW lines = %v, want 2 (header + checks)", section.Lines)
	}
	for _, line := range section.Lines {
		if !strings.HasPrefix(line, domain.DetailListIndent) {
			t.Errorf("ligne REVIEW = %q, want le préfixe %q comme CHANGES/ACTIVITY/LINKS", line, domain.DetailListIndent)
		}
	}
}

// TestActivityTitleRightShowsBranchDiff pins that ACTIVITY mirrors CHANGES:
// the committed diff volume against the base branch, plus the file count
// `git diff --shortstat` reports alongside it, show on the title row.
func TestActivityTitleRightShowsBranchDiff(t *testing.T) {
	section := activitySection(activitySectionParams{
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
		Budget:  domain.DashboardDetailCommits,
		Loaded:  true,
		Diff:    domain.DiffStat{FilesChanged: 7, Insertions: 214, Deletions: 38},
	})
	if !strings.Contains(section.TitleRight, "214") || !strings.Contains(section.TitleRight, "38") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the committed diff volume", section.TitleRight)
	}
	if !strings.Contains(section.TitleRight, "7 files changed") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the file count alongside it, like CHANGES", section.TitleRight)
	}
}

// TestActivityTitleRightSaysWhyOnDiffFailure pins that a diff-stat read
// failure never fabricates a zero: it says why, like every other family.
func TestActivityTitleRightSaysWhyOnDiffFailure(t *testing.T) {
	section := activitySection(activitySectionParams{
		Commits:     []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
		Budget:      domain.DashboardDetailCommits,
		Loaded:      true,
		DiffFailure: errors.New("git diff failed"),
	})
	if !strings.Contains(section.TitleRight, "git diff failed") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the failure reason, not a fabricated zero", section.TitleRight)
	}
}

func TestRunSectionLeadsThePanelAndNamesEveryDeclaredJob(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		DetailLoaded: true,
		Height:       60,
		Now:          time.Now(),
		RunConfig:    domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}},
		Jobs: []domain.JobInfo{{
			Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x",
			StartedAt: time.Now().Add(-4 * time.Minute),
		}},
		Detail: domain.WorktreeDetail{RunAddresses: map[string]domain.JobAddress{
			"web": {Ports: []int{3000}, URL: "http://web.wtm"},
		}},
	})

	if sections[0].Key != domain.DetailSectionRun {
		t.Fatalf("first section = %q, want RUN in the lead", sections[0].Key)
	}
	if want := fmt.Sprintf(domain.DetailRunCountFmt, 1); sections[0].TitleRight != want {
		t.Errorf("TitleRight = %q, want %q", sections[0].TitleRight, want)
	}
	body := strings.Join(sections[0].Lines, "\n")
	for _, want := range []string{"web", ":3000", "http://web.wtm", "worker", domain.DetailJobStopped} {
		if !strings.Contains(body, want) {
			t.Errorf("RUN body %q misses %q", body, want)
		}
	}
}

// A job of the same name in another worktree is not this one: the daemon
// indexes every repository it knows.
func TestRunSectionIgnoresAnotherWorktreesJobs(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs:      []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/other"}},
	})

	if sections[0].TitleRight != domain.DetailRunNothing {
		t.Fatalf("TitleRight = %q, want nothing counted here", sections[0].TitleRight)
	}
}

func TestRunSectionSaysNothingIsUpRatherThanVanishing(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
	})

	if sections[0].TitleRight != domain.DetailRunNothing {
		t.Errorf("TitleRight = %q, want the section to say what it is worth", sections[0].TitleRight)
	}
	if len(sections[0].Lines) != 1 {
		t.Errorf("lines = %v, want the declared job listed as down", sections[0].Lines)
	}
}

func TestRunSectionIsAbsentWhenTheProjectHasNoRun(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x"}, DetailLoaded: true, Height: 60, Now: time.Now(),
	})

	for _, section := range sections {
		if section.Key == domain.DetailSectionRun {
			t.Fatal("no run configured is a legitimate absence, not a state to announce")
		}
	}
}

func TestRunSectionFoldsWhatItCannotShow(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, domain.DashboardDetailJobs+3)
	for index := range domain.DashboardDetailJobs + 3 {
		jobs = append(jobs, domain.JobConfig{Name: fmt.Sprintf("job%d", index)})
	}

	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x"}, DetailLoaded: true, Height: 80, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: jobs},
	})

	if got := len(sections[0].Lines); got != domain.DashboardDetailJobs {
		t.Fatalf("RUN has %d lines, want the cap, its « more » line included", got)
	}
	if last := sections[0].Lines[len(sections[0].Lines)-1]; !strings.Contains(last, "4") {
		t.Errorf("last line = %q, want it to name what was folded", last)
	}
}

// RUN is the most volatile thing the panel holds, so it leads it — and gives up
// its place last when the height runs short.
func TestRunSectionIsTheLastToFallWhenTheHeightRunsShort(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 8, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
	})
	kept := FitSections(FitSectionsParams{Sections: sections, Height: 6})

	if len(kept) != 1 || kept[0].Key != domain.DetailSectionRun {
		t.Fatalf("kept = %+v, want RUN alone standing", kept)
	}
}

// A job dropped from run.toml while it is still up has no line here, so the
// header must not count it: a count naming something the body does not show
// reads as a section that lost a row.
func TestRunSectionCountsOnlyWhatItCouldList(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x"},
			{Name: "gone", Status: domain.JobStatusRunning, WorkDir: "/wt/x"},
		},
	})

	if want := fmt.Sprintf(domain.DetailRunCountFmt, 1); sections[0].TitleRight != want {
		t.Fatalf("TitleRight = %q, want %q — only the declared job has a line", sections[0].TitleRight, want)
	}
}

// A folded job is still up, and the count says so: the « … N more » line is
// what explains the difference between the header and the rows.
func TestRunSectionCountsWhatItFolded(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, domain.DashboardDetailJobs+2)
	infos := make([]domain.JobInfo, 0, domain.DashboardDetailJobs+2)
	for index := range domain.DashboardDetailJobs + 2 {
		name := fmt.Sprintf("job%d", index)
		jobs = append(jobs, domain.JobConfig{Name: name})
		infos = append(infos, domain.JobInfo{Name: name, Status: domain.JobStatusRunning, WorkDir: "/wt/x"})
	}

	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 80, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: jobs}, Jobs: infos,
	})

	if want := fmt.Sprintf(domain.DetailRunCountFmt, len(jobs)); sections[0].TitleRight != want {
		t.Fatalf("TitleRight = %q, want %q", sections[0].TitleRight, want)
	}
}
