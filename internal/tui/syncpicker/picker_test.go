package syncpicker

import (
	"testing"

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

func TestKeepConflictFromPrev(t *testing.T) {
	keepStep := components.Step{Model: conflictModeStep(conflictModeStepParams{DefaultKeep: true})}
	normalStep := components.Step{Model: conflictModeStep(conflictModeStepParams{DefaultKeep: false})}
	placeholder := components.Step{Model: nil}

	tests := []struct {
		name string
		prev []components.Step
		want bool
	}{
		{name: "keep selected", prev: []components.Step{placeholder, keepStep}, want: true},
		{name: "normal selected", prev: []components.Step{placeholder, normalStep}, want: false},
		{name: "missing conflict step", prev: []components.Step{placeholder}, want: false},
		{name: "wrong model type", prev: []components.Step{placeholder, placeholder}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := keepConflictFromPrev(tc.prev); got != tc.want {
				t.Fatalf("keepConflictFromPrev() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfirmStep_KeepConflictShowsWarning(t *testing.T) {
	warned := confirmStep(confirmStepParams{PlanText: "Sync plan", Count: 2, KeepConflict: true})
	if warned.View() == "" {
		t.Fatal("expected a rendered confirm view")
	}

	// The danger warning only shows when conflicts are kept in progress.
	kept := confirmStep(confirmStepParams{PlanText: "Sync plan", Count: 2, KeepConflict: true}).View()
	clean := confirmStep(confirmStepParams{PlanText: "Sync plan", Count: 2, KeepConflict: false}).View()
	if len(kept) <= len(clean) {
		t.Errorf("keep-conflict view should include a warning banner (longer), got kept=%d clean=%d", len(kept), len(clean))
	}
}
