package syncpicker

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// The on-conflict step announces the cascade before the plan recap is shown, so
// it must not name the base branch as the rebase target: a nested worktree is
// rebased onto its own parent, not onto the base (see evaluateStep's NewBase).
func TestSyncCounter(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{name: "several worktrees", count: 3, want: "About to sync 3 worktree(s) onto their parent."},
		{name: "single worktree", count: 1, want: "About to sync 1 worktree(s) onto their parent."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncCounter(tc.count); got != tc.want {
				t.Fatalf("syncCounter(%d) = %q, want %q", tc.count, got, tc.want)
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

// The parent question lives inside the wizard, so it must disappear on its own
// when the current selection leaves no stale parent — not be asked and answered
// "no" every run.
func TestParentStepAutoSkipsWithoutStaleParents(t *testing.T) {
	step, asked := parentStep(parentStepParams{
		Preselected: []string{"dev-1"},
		Stale:       func([]string) []domain.ParentUpdate { return nil },
	})

	step.Model = step.Build(nil)
	if !step.AutoSkip(components.WizardModel{}) {
		t.Fatal("the parent step should auto-skip when nothing is stale")
	}
	if *asked {
		t.Fatal("a skipped step must not report itself as asked")
	}
	// A skipped step still holds the model Build left behind, whose leading option
	// is the fast-forward — so its value alone would read back as an answer nobody
	// gave. This is exactly why Run gates the result on the asked flag.
	if !resolveParentFF([]components.Step{step}) {
		t.Fatal("precondition: the built model's value is the leading fast-forward")
	}
}

func TestParentStepAsksWhenAParentIsStale(t *testing.T) {
	var received []string
	step, asked := parentStep(parentStepParams{
		Preselected: []string{"dev-1", "dev-2"},
		Stale: func(branches []string) []domain.ParentUpdate {
			received = branches
			return []domain.ParentUpdate{
				{Branch: "feature", Behind: 2, Children: []string{"dev-1", "dev-2"}},
			}
		},
	})

	step.Model = step.Build(nil)
	if step.AutoSkip(components.WizardModel{}) {
		t.Fatal("the parent step must be shown when a parent is stale")
	}
	if !*asked {
		t.Fatal("a shown step should report itself as asked")
	}
	// With no multi-select step before it, the hook reads the preselected branches.
	if len(received) != 2 || received[0] != "dev-1" {
		t.Fatalf("hook received %v, want the preselected branches", received)
	}
	// Fast-forward leads, so it is the highlighted default.
	if !resolveParentFF([]components.Step{step}) {
		t.Error("fast-forward should be the leading choice")
	}
}

// The parent step must read like the on-conflict step: a situation line, then the
// reason, then two explicit "action — consequence" choices.
func TestParentStepWording(t *testing.T) {
	lines := parentLines([]domain.ParentUpdate{
		{Branch: "feature", Behind: 2, Children: []string{"dev-1", "dev-2"}},
		{Branch: "legacy", Behind: 1, Children: []string{"dev-3"}},
	})

	want := "feature is 2 commits behind origin/feature — dev-1, dev-2 rebase onto it.\n" +
		"legacy is 1 commit behind origin/legacy — dev-3 rebase onto it."
	if lines != want {
		t.Fatalf("parentLines =\n%s\nwant:\n%s", lines, want)
	}
}

func TestParentSummaryLabelsTheChoice(t *testing.T) {
	stale := []domain.ParentUpdate{{Branch: "feature", Behind: 1, Children: []string{"dev"}}}
	if got := parentSummary(parentModeStep(stale)); got != "fast-forward" {
		t.Fatalf("summary = %q, want %q", got, "fast-forward")
	}
}
