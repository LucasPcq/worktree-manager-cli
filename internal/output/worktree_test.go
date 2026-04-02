package output

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func init() {
	os.Setenv("NO_COLOR", "1")
}

func TestFormatWorktreeListEmpty(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{})
	if got != "No worktrees found." {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestFormatWorktreeListSingleParent(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, IsDirty: false, CommitsAhead: 0, CreatedAt: time.Now()},
		},
	})

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
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, IsDirty: false, CreatedAt: time.Now()},
			{Branch: "feature-auth", IsParent: false, IsDirty: true, CommitsAhead: 3, CreatedAt: time.Now()},
		},
	})

	if !strings.Contains(got, "dirty") {
		t.Error("expected output to contain 'dirty'")
	}
	if !strings.Contains(got, "3 commits ahead") {
		t.Error("expected output to contain '3 commits ahead'")
	}
}

func TestFormatWorktreeListSingleCommitAhead(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "fix", IsParent: false, IsDirty: false, CommitsAhead: 1, CreatedAt: time.Now()},
		},
	})

	if !strings.Contains(got, "1 commit ahead") {
		t.Error("expected '1 commit ahead' (singular)")
	}
}

func TestFormatWorktreeListActiveIndicator(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, CreatedAt: time.Now()},
			{Branch: "feature/auth", IsParent: false, CreatedAt: time.Now()},
		},
		ActiveBranch: "feature/auth",
	})

	if !strings.Contains(got, "active") {
		t.Error("expected active indicator on focused worktree")
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

func TestFormatTagParentOnly(t *testing.T) {
	got := formatTag(true, false)
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if strings.Contains(got, "active") {
		t.Error("expected output to NOT contain 'active'")
	}
}

func TestFormatTagActiveOnly(t *testing.T) {
	got := formatTag(false, true)
	if !strings.Contains(got, "active") {
		t.Error("expected output to contain 'active'")
	}
	if strings.Contains(got, "(parent)") {
		t.Error("expected output to NOT contain '(parent)'")
	}
}

func TestFormatTagBoth(t *testing.T) {
	got := formatTag(true, true)
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if !strings.Contains(got, "active") {
		t.Error("expected output to contain 'active'")
	}
}

func TestFormatTagNeither(t *testing.T) {
	got := formatTag(false, false)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatAheadZero(t *testing.T) {
	got := formatAhead(0)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatAheadOne(t *testing.T) {
	got := formatAhead(1)
	if !strings.Contains(got, "1 commit ahead") {
		t.Errorf("expected '1 commit ahead' (singular), got %q", got)
	}
}

func TestFormatAheadMultiple(t *testing.T) {
	got := formatAhead(5)
	if !strings.Contains(got, "5 commits ahead") {
		t.Errorf("expected '5 commits ahead' (plural), got %q", got)
	}
}

func TestPrintableLenEmpty(t *testing.T) {
	if printableLen("") != 0 {
		t.Error("expected printableLen of empty string to be 0")
	}
}

func TestAnsiOverheadPlainString(t *testing.T) {
	if ansiOverhead("hello world") != 0 {
		t.Error("expected ansiOverhead of plain string to be 0")
	}
}
