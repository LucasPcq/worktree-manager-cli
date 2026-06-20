package wt

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestFindPRForBranch(t *testing.T) {
	prs := []domain.PRInfo{
		{Number: 1, Branch: "feature/a", URL: "https://example.com/1"},
		{Number: 2, Branch: "feature/b", URL: "https://example.com/2"},
	}

	pr, ok := findPRForBranch(prs, "feature/b")
	if !ok || pr.Number != 2 {
		t.Fatalf("expected PR #2 for feature/b, got %+v ok=%v", pr, ok)
	}

	if _, ok := findPRForBranch(prs, "feature/missing"); ok {
		t.Error("expected no PR for missing branch")
	}
}

func TestBuildActionItems_OpenPRGating(t *testing.T) {
	prs := []domain.PRInfo{{Number: 7, Branch: "feature/a", URL: "https://example.com/7"}}

	cases := []struct {
		name         string
		branch       string
		conn         domain.GHConnection
		wantDisabled bool
	}{
		{"pr present and connected", "feature/a", domain.GHConnectionOK, false},
		{"no pr for branch", "feature/none", domain.GHConnectionOK, true},
		{"pr present but gh not authenticated", "feature/a", domain.GHConnectionNotAuthenticated, true},
		{"pr present but gh not installed", "feature/a", domain.GHConnectionNotInstalled, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := buildActionItems(buildActionItemsParams{
				selected:  domain.WorktreeStatus{Branch: tc.branch},
				prs:       prs,
				conn:      tc.conn,
				prsLoaded: true,
			})

			found := false
			disabled := false
			for _, it := range items {
				if it.Value == lsActionOpenPR {
					found = true
					disabled = it.Disabled
				}
			}
			if !found {
				t.Fatal("Open PR action missing from menu")
			}
			if disabled != tc.wantDisabled {
				t.Errorf("Open PR disabled = %v, want %v", disabled, tc.wantDisabled)
			}
		})
	}
}

// While PRs are still streaming in, Open PR stays enabled optimistically; the
// action resolves the URL per-branch at execution time.
func TestBuildActionItems_OpenPROptimisticWhileLoading(t *testing.T) {
	items := buildActionItems(buildActionItemsParams{
		selected:  domain.WorktreeStatus{Branch: "feature/none"},
		prsLoaded: false,
	})

	found := false
	for _, it := range items {
		if it.Value == lsActionOpenPR {
			found = true
			if it.Disabled {
				t.Error("Open PR should stay enabled while PRs are loading")
			}
		}
	}
	if !found {
		t.Fatal("Open PR action missing from menu")
	}
}
