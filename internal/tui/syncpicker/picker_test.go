package syncpicker

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestSyncCounter(t *testing.T) {
	tests := []struct {
		name  string
		count int
		base  string
		want  string
	}{
		{name: "with base", count: 3, base: "main", want: "About to sync 3 worktree(s) onto main."},
		{name: "single with base", count: 1, base: "develop", want: "About to sync 1 worktree(s) onto develop."},
		{name: "no base", count: 2, base: "", want: "About to sync 2 worktree(s)."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncCounter(tc.count, tc.base); got != tc.want {
				t.Fatalf("syncCounter(%d, %q) = %q, want %q", tc.count, tc.base, got, tc.want)
			}
		})
	}
}

func TestResolveBranches(t *testing.T) {
	// No multi-select step → fall back to the preselected branches (args / --all).
	if got := resolveBranches(nil, []string{"a", "b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("resolveBranches(nil, preselected) = %v, want the preselected list", got)
	}

	// A multi-select step → its checked values take precedence.
	ms := components.NewMultiSelect(components.NewMultiSelectParams{
		Items: []components.MultiSelectItem{{Value: "x", Selected: true}},
	})
	steps := []components.Step{{Name: stepWorktrees, Model: ms}}
	if got := resolveBranches(steps, []string{"fallback"}); len(got) != 1 || got[0] != "x" {
		t.Errorf("resolveBranches(withSelect, _) = %v, want [x]", got)
	}
}

func TestResolveKeep(t *testing.T) {
	keepStep := components.Step{Name: stepConflict, Model: conflictModeStep(conflictModeStepParams{DefaultKeep: true})}
	normalStep := components.Step{Name: stepConflict, Model: conflictModeStep(conflictModeStepParams{DefaultKeep: false})}

	tests := []struct {
		name     string
		steps    []components.Step
		fallback bool
		want     bool
	}{
		{name: "keep selected", steps: []components.Step{keepStep}, want: true},
		{name: "normal selected", steps: []components.Step{normalStep}, want: false},
		{name: "step absent falls back to flag", steps: nil, fallback: true, want: true},
		{name: "step absent, no flag", steps: nil, fallback: false, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveKeep(tc.steps, tc.fallback); got != tc.want {
				t.Fatalf("resolveKeep() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfirmDescription_KeepConflictFoldsWarning(t *testing.T) {
	kept := confirmDescription(confirmStepParams{PlanText: "Sync plan", Count: 2, KeepConflict: true})
	clean := confirmDescription(confirmStepParams{PlanText: "Sync plan", Count: 2, KeepConflict: false})

	// The danger warning is folded into the recap description as a ⚠ line only when
	// conflicts are kept in progress.
	if !strings.Contains(kept, domain.SyncKeepConflictWarning) {
		t.Errorf("keep-conflict recap should fold in the warning, got:\n%s", kept)
	}
	if strings.Contains(clean, domain.SyncKeepConflictWarning) {
		t.Errorf("clean recap should not include the keep-conflict warning, got:\n%s", clean)
	}
	if !strings.Contains(kept, "Sync plan") {
		t.Errorf("recap should include the plan text, got:\n%s", kept)
	}
}
