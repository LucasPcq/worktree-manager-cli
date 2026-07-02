package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestConflictFileList_CapsAtFive(t *testing.T) {
	files := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := conflictFileList(files)
	if !strings.HasPrefix(got, "a, b, c, d, e") {
		t.Fatalf("expected first five listed, got %q", got)
	}
	if !strings.Contains(got, "…+2 more") {
		t.Fatalf("expected collapsed remainder, got %q", got)
	}
}

func TestConflictFileList_ShortListed(t *testing.T) {
	if got := conflictFileList([]string{"a", "b"}); got != "a, b" {
		t.Fatalf("got %q, want %q", got, "a, b")
	}
}

func TestFormatSyncResult_KeptConflictShowsFooter(t *testing.T) {
	result := domain.SyncResult{
		BaseBranch: "main",
		Steps: []domain.SyncStepResult{
			{
				Branch:         "feat",
				SourceBranch:   "main",
				Path:           "/tmp/wt/feat",
				Status:         domain.SyncStatusConflict,
				ConflictFiles:  []string{"a.go", "b.go"},
				KeptInProgress: true,
			},
		},
	}

	var buf bytes.Buffer
	FormatSyncResult(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "left in progress (2 files)") {
		t.Errorf("expected enriched conflict line, got:\n%s", out)
	}
	if !strings.Contains(out, "Conflicts left in progress") {
		t.Errorf("expected footer header, got:\n%s", out)
	}
	if !strings.Contains(out, "/tmp/wt/feat") {
		t.Errorf("expected worktree path in footer, got:\n%s", out)
	}
	if !strings.Contains(out, "git rebase --continue") {
		t.Errorf("expected resume hint, got:\n%s", out)
	}
	if !strings.Contains(out, "a.go, b.go") {
		t.Errorf("expected conflicting files list, got:\n%s", out)
	}
}

func TestSprintSyncPlan_EmptyPlan(t *testing.T) {
	if got := SprintSyncPlan(domain.SyncPlan{BaseBranch: "main"}); got != "" {
		t.Fatalf("empty plan should render empty, got %q", got)
	}
}

func TestSprintSyncPlan_ListsStepsPlain(t *testing.T) {
	plan := domain.SyncPlan{
		BaseBranch: "main",
		Steps: []domain.SyncStep{
			{Branch: "feat/a", SourceBranch: "main"},
			{Branch: "feat/b", SourceBranch: "feat/a"},
		},
	}

	got := SprintSyncPlan(plan)

	want := "Sync plan (base: main)\n1. feat/a ← main\n2. feat/b ← feat/a"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	// Plain output: no ANSI escapes, so it can be re-styled inside a wizard desc.
	if strings.Contains(got, "\x1b[") {
		t.Errorf("expected plain (unstyled) output, got ANSI escapes: %q", got)
	}
}

func TestSprintSyncPlan_UnknownParent(t *testing.T) {
	plan := domain.SyncPlan{
		BaseBranch: "main",
		Steps:      []domain.SyncStep{{Branch: "feat/a"}},
	}
	if got := SprintSyncPlan(plan); !strings.Contains(got, "unknown parent") {
		t.Fatalf("expected 'unknown parent' fallback, got %q", got)
	}
}

func TestFormatWorktreeState_RebasingPrecedesDirty(t *testing.T) {
	rebasing := formatWorktreeState(domain.WorktreeStatus{IsDirty: true, RebaseInProgress: true})
	if !strings.Contains(rebasing, "rebasing") {
		t.Errorf("expected 'rebasing' badge, got %q", rebasing)
	}
	if strings.Contains(rebasing, "dirty") {
		t.Errorf("rebasing should take precedence over dirty, got %q", rebasing)
	}

	if dirty := formatWorktreeState(domain.WorktreeStatus{IsDirty: true}); !strings.Contains(dirty, "dirty") {
		t.Errorf("expected 'dirty' badge, got %q", dirty)
	}
	if clean := formatWorktreeState(domain.WorktreeStatus{}); !strings.Contains(clean, "clean") {
		t.Errorf("expected 'clean' badge, got %q", clean)
	}
}

func TestFormatSyncResult_AbortedConflictNoFooter(t *testing.T) {
	result := domain.SyncResult{
		BaseBranch: "main",
		Steps: []domain.SyncStepResult{
			{
				Branch:        "feat",
				SourceBranch:  "main",
				Status:        domain.SyncStatusConflict,
				ConflictFiles: []string{"a.go", "b.go"},
			},
		},
	}

	var buf bytes.Buffer
	FormatSyncResult(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "aborted, tree clean (2 files: a.go, b.go)") {
		t.Errorf("expected enriched aborted line, got:\n%s", out)
	}
	if strings.Contains(out, "Conflicts left in progress") {
		t.Errorf("did not expect footer in aborted mode, got:\n%s", out)
	}
}
