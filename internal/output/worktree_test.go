package output

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func init() {
	// Disable colors for deterministic test output
	os.Setenv("NO_COLOR", "1")
}

func TestFormatWorktreeListEmpty(t *testing.T) {
	got := FormatWorktreeList(nil)
	if got != "No worktrees found." {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestFormatWorktreeListSingleParent(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", IsParent: true, IsDirty: false, CommitsAhead: 0, CreatedAt: time.Now()},
	}

	got := FormatWorktreeList(statuses)

	if !strings.Contains(got, "main") {
		t.Error("expected output to contain 'main'")
	}
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if !strings.Contains(got, "clean") {
		t.Error("expected output to contain 'clean'")
	}
}

func TestFormatWorktreeListDirtyAndAhead(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", IsParent: true, IsDirty: false, CreatedAt: time.Now()},
		{Branch: "feature-auth", IsParent: false, IsDirty: true, CommitsAhead: 3, CreatedAt: time.Now()},
	}

	got := FormatWorktreeList(statuses)

	if !strings.Contains(got, "dirty") {
		t.Error("expected output to contain 'dirty'")
	}
	if !strings.Contains(got, "3 commits ahead") {
		t.Error("expected output to contain '3 commits ahead'")
	}
}

func TestFormatWorktreeListSingleCommitAhead(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "fix", IsParent: false, IsDirty: false, CommitsAhead: 1, CreatedAt: time.Now()},
	}

	got := FormatWorktreeList(statuses)

	if !strings.Contains(got, "1 commit ahead") {
		t.Error("expected '1 commit ahead' (singular)")
	}
}

func TestPrintableLen(t *testing.T) {
	if printableLen("hello") != 5 {
		t.Error("plain string length wrong")
	}
	if printableLen("\x1b[1mhello\x1b[0m") != 5 {
		t.Error("ANSI string length wrong")
	}
}
