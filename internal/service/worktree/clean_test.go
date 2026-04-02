package worktree

import "testing"

func TestHasWarnings_NoWarnings(t *testing.T) {
	result := CleanCheckResult{}

	if result.HasWarnings() {
		t.Error("expected HasWarnings() to return false for zero-value result")
	}
}

func TestHasWarnings_IsDirty(t *testing.T) {
	result := CleanCheckResult{IsDirty: true}

	if !result.HasWarnings() {
		t.Error("expected HasWarnings() to return true when IsDirty is true")
	}
}

func TestHasWarnings_UnpushedCommits(t *testing.T) {
	result := CleanCheckResult{UnpushedCommits: 3}

	if !result.HasWarnings() {
		t.Error("expected HasWarnings() to return true when UnpushedCommits > 0")
	}
}

func TestHasWarnings_HasOpenPR(t *testing.T) {
	result := CleanCheckResult{HasOpenPR: true}

	if !result.HasWarnings() {
		t.Error("expected HasWarnings() to return true when HasOpenPR is true")
	}
}

func TestHasWarnings_MultipleWarnings(t *testing.T) {
	result := CleanCheckResult{
		IsDirty:         true,
		UnpushedCommits: 2,
		HasOpenPR:       true,
	}

	if !result.HasWarnings() {
		t.Error("expected HasWarnings() to return true when multiple warnings are present")
	}
}
