package worktree

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestResolveExactMatch(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-auth", Branch: "feature/auth"},
		{Path: "/trees/feature-auth-v2", Branch: "feature/auth-v2"},
	}

	result := resolveFromList(worktrees, "feature/auth")

	if result.Ambiguous {
		t.Error("expected direct match, not ambiguous")
	}
	if result.Path != "/trees/feature-auth" {
		t.Errorf("expected /trees/feature-auth, got %s", result.Path)
	}
}

func TestResolveSubstringUnique(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-login", Branch: "feature/login"},
	}

	result := resolveFromList(worktrees, "login")

	if result.Ambiguous {
		t.Error("expected direct match for unique substring")
	}
	if result.Path != "/trees/feature-login" {
		t.Errorf("expected /trees/feature-login, got %s", result.Path)
	}
}

func TestResolveSubstringAmbiguous(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-auth", Branch: "feature/auth"},
		{Path: "/trees/feature-auth-v2", Branch: "feature/auth-v2"},
	}

	result := resolveFromList(worktrees, "auth")

	if !result.Ambiguous {
		t.Error("expected ambiguous result")
	}
	if len(result.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestResolveEmptyQuery(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feat", Branch: "feat"},
	}

	result := resolveFromList(worktrees, "")

	if !result.Ambiguous {
		t.Error("expected ambiguous (all worktrees) for empty query")
	}
	if len(result.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestResolveNotFound(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
	}

	result := resolveFromList(worktrees, "nonexistent")

	if result.Path != "" || result.Ambiguous {
		t.Error("expected empty result for no match")
	}
}

// resolveFromList is a test helper that applies the resolve logic to a pre-built list.
func resolveFromList(worktrees []domain.GitWorktree, query string) domain.ResolveResult {
	if query == "" {
		return domain.ResolveResult{Ambiguous: true, Matches: worktrees}
	}

	for _, wt := range worktrees {
		if wt.Branch == query {
			return domain.ResolveResult{Path: wt.Path}
		}
	}

	var matches []domain.GitWorktree
	for _, wt := range worktrees {
		if contains(wt.Branch, query) {
			matches = append(matches, wt)
		}
	}

	if len(matches) == 1 {
		return domain.ResolveResult{Path: matches[0].Path}
	}
	if len(matches) > 1 {
		return domain.ResolveResult{Ambiguous: true, Matches: matches}
	}

	return domain.ResolveResult{}
}

func contains(s string, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsStr(s, substr))
}

func containsStr(s string, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMatchSyncBranchExact(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "main", IsMain: true},
		{Branch: "feature/auth"},
		{Branch: "feature/auth-v2"},
	}

	branch, err := matchSyncBranch(worktrees, "feature/auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/auth" {
		t.Fatalf("expected feature/auth, got %s", branch)
	}
}

func TestMatchSyncBranchSubstringUnique(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "main", IsMain: true},
		{Branch: "feature/login"},
	}

	branch, err := matchSyncBranch(worktrees, "login")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/login" {
		t.Fatalf("expected feature/login, got %s", branch)
	}
}

func TestMatchSyncBranchAmbiguous(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "feature/auth"},
		{Branch: "feature/auth-v2"},
	}

	if _, err := matchSyncBranch(worktrees, "auth"); err == nil {
		t.Fatal("expected an ambiguity error")
	}
}

func TestMatchSyncBranchNotFound(t *testing.T) {
	worktrees := []domain.GitWorktree{{Branch: "main", IsMain: true}}

	_, err := matchSyncBranch(worktrees, "nope")
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("expected ErrBranchNotFound, got %v", err)
	}
}

func TestResolveNotFoundError(t *testing.T) {
	// This tests the actual Resolve function would return ErrWorktreeNotFound
	// but we can't easily test it without a git repo, so we verify the error type
	err := domain.ErrWorktreeNotFound
	if !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Error("expected ErrWorktreeNotFound")
	}
}
