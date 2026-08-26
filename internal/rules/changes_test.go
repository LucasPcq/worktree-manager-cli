package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestCountChanges(t *testing.T) {
	entries := []domain.PorcelainEntry{
		{Status: " M", Path: "a.go"},
		{Status: "M ", Path: "b.go"},
		{Status: "MM", Path: "c.go"},
		{Status: "??", Path: "d.go"},
		{Status: "A ", Path: "e.go"},
		{Status: " D", Path: "f.go"},
	}

	got := CountChanges(entries)

	if got.Modified != 3 {
		t.Errorf("Modified = %d, want 3 (' M', 'MM', ' D')", got.Modified)
	}
	if got.Staged != 3 {
		t.Errorf("Staged = %d, want 3 ('M ', 'MM', 'A ')", got.Staged)
	}
	if got.Untracked != 1 {
		t.Errorf("Untracked = %d, want 1", got.Untracked)
	}
	if len(got.Files) != len(entries) {
		t.Errorf("Files = %d, want %d", len(got.Files), len(entries))
	}
}

func TestCountChangesEmpty(t *testing.T) {
	got := CountChanges(nil)
	if got.Modified != 0 || got.Staged != 0 || got.Untracked != 0 || len(got.Files) != 0 {
		t.Errorf("CountChanges(nil) = %+v, want zéro partout", got)
	}
}
