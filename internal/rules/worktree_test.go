package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestParentEnvFallsBackToMain(t *testing.T) {
	cases := []struct {
		name   string
		params ParentEnvFallbackParams
		want   bool
	}{
		{"parent + no worktree + copyfiles", ParentEnvFallbackParams{Strategy: domain.EnvStrategyParent, HasCopyFiles: true, SourceHasWorktree: false}, true},
		{"parent + has worktree", ParentEnvFallbackParams{Strategy: domain.EnvStrategyParent, HasCopyFiles: true, SourceHasWorktree: true}, false},
		{"parent + no copyfiles", ParentEnvFallbackParams{Strategy: domain.EnvStrategyParent, HasCopyFiles: false, SourceHasWorktree: false}, false},
		{"main strategy", ParentEnvFallbackParams{Strategy: domain.EnvStrategyMain, HasCopyFiles: true, SourceHasWorktree: false}, false},
		{"example strategy", ParentEnvFallbackParams{Strategy: domain.EnvStrategyExample, HasCopyFiles: true, SourceHasWorktree: false}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParentEnvFallsBackToMain(c.params); got != c.want {
				t.Errorf("ParentEnvFallsBackToMain(%+v) = %v, want %v", c.params, got, c.want)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feature/login", "feature-login"},
		{"fix/auth/step-2", "fix-auth-step-2"},
		{"simple", "simple"},
		{"a/b/c/d", "a-b-c-d"},
	}

	for _, tt := range tests {
		got := SanitizeBranchName(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		name   string
		dir    string
		target string
		want   bool
	}{
		{"equal", "/repo/.trees/feat", "/repo/.trees/feat", true},
		{"nested", "/repo/.trees/feat", "/repo/.trees/feat/src/main.go", true},
		{"sibling", "/repo/.trees/feat", "/repo/.trees/other", false},
		{"outside", "/repo/.trees/feat", "/repo", false},
		{"unrelated", "/repo/.trees/feat", "/tmp/x", false},
		{"prefix-not-segment", "/repo/feat", "/repo/feature", false},
	}

	for _, tt := range tests {
		if got := IsPathWithin(tt.dir, tt.target); got != tt.want {
			t.Errorf("IsPathWithin(%q, %q) = %v, want %v", tt.dir, tt.target, got, tt.want)
		}
	}
}

func TestResolveEnvStrategy(t *testing.T) {
	if got := ResolveEnvStrategy(domain.EnvStrategyExample, ""); got != domain.EnvStrategyExample {
		t.Errorf("expected example, got %s", got)
	}
	if got := ResolveEnvStrategy(domain.EnvStrategyExample, "parent"); got != domain.EnvStrategyParent {
		t.Errorf("expected parent override, got %s", got)
	}
}

func TestSortStatusesParentFirst(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "feature", IsParent: false, CreatedAt: time.Now()},
		{Branch: "main", IsParent: true, CreatedAt: time.Now()},
	}

	SortStatuses(statuses)

	if statuses[0].Branch != "main" {
		t.Errorf("expected parent (main) first, got %s", statuses[0].Branch)
	}
}

func TestSortStatusesByCreationDate(t *testing.T) {
	now := time.Now()

	statuses := []domain.WorktreeStatus{
		{Branch: "newer", IsParent: false, CreatedAt: now.Add(2 * time.Hour)},
		{Branch: "main", IsParent: true, CreatedAt: now},
		{Branch: "older", IsParent: false, CreatedAt: now.Add(1 * time.Hour)},
	}

	SortStatuses(statuses)

	if statuses[0].Branch != "main" {
		t.Errorf("expected main first, got %s", statuses[0].Branch)
	}
	if statuses[1].Branch != "older" {
		t.Errorf("expected older second, got %s", statuses[1].Branch)
	}
	if statuses[2].Branch != "newer" {
		t.Errorf("expected newer third, got %s", statuses[2].Branch)
	}
}

func TestHasWarnings_NoWarnings(t *testing.T) {
	result := domain.CleanCheckResult{}

	if HasWarnings(result) {
		t.Error("expected HasWarnings to return false for zero-value result")
	}
}

func TestHasWarnings_IsDirty(t *testing.T) {
	result := domain.CleanCheckResult{IsDirty: true}

	if !HasWarnings(result) {
		t.Error("expected HasWarnings to return true when IsDirty is true")
	}
}

func TestHasWarnings_UnpushedCommits(t *testing.T) {
	result := domain.CleanCheckResult{UnpushedCommits: 3}

	if !HasWarnings(result) {
		t.Error("expected HasWarnings to return true when UnpushedCommits > 0")
	}
}

func TestHasWarnings_HasOpenPR(t *testing.T) {
	result := domain.CleanCheckResult{HasOpenPR: true}

	if !HasWarnings(result) {
		t.Error("expected HasWarnings to return true when HasOpenPR is true")
	}
}

func TestHasWarnings_MultipleWarnings(t *testing.T) {
	result := domain.CleanCheckResult{
		IsDirty:         true,
		UnpushedCommits: 2,
		HasOpenPR:       true,
	}

	if !HasWarnings(result) {
		t.Error("expected HasWarnings to return true when multiple warnings are present")
	}
}
