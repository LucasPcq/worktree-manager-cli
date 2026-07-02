package extract

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/tui/components"
)

func targetStepWith(value string) components.Step {
	return components.Step{
		Name:  stepTarget,
		Model: components.NewSelectList(components.NewSelectListParams{Items: []components.SelectItem{{Label: value, Value: value}}}),
	}
}

func TestIsNewTarget(t *testing.T) {
	if !isNewTarget([]components.Step{targetStepWith(newWorktreeValue)}) {
		t.Error("expected the create-new sentinel to be recognized as a new target")
	}
	if isNewTarget([]components.Step{targetStepWith("feat-a")}) {
		t.Error("an existing branch is not a new target")
	}
	if isNewTarget(nil) {
		t.Error("no target step → not a new target")
	}
}

func TestBuildCombinedRecap_ExistingTarget(t *testing.T) {
	files := components.NewMultiSelect(components.NewMultiSelectParams{
		Items: []components.MultiSelectItem{{Value: "a.txt", Selected: true}},
	})
	mode := components.NewSelectList(components.NewSelectListParams{
		Items: []components.SelectItem{{Label: "Move", Value: modeMove}},
	})
	prev := []components.Step{
		{Name: stepFiles, Model: files},
		targetStepWith("feat-a"),
		{Name: stepMode, Model: mode},
	}

	rc := buildCombinedRecap(prev, RunParams{})
	if !strings.Contains(rc.Description, "a.txt") {
		t.Errorf("recap should list the files, got:\n%s", rc.Description)
	}
	if !strings.Contains(rc.Description, "feat-a") {
		t.Errorf("recap should name the existing target, got:\n%s", rc.Description)
	}
	if rc.Actions[0].Label != "Yes, extract" {
		t.Errorf("existing target → action %q, want \"Yes, extract\"", rc.Actions[0].Label)
	}
}

func TestBuildCombinedRecap_NewTargetActionLabel(t *testing.T) {
	prev := []components.Step{targetStepWith(newWorktreeValue)}
	rc := buildCombinedRecap(prev, RunParams{})
	if rc.Actions[0].Label != "Yes, create & extract" {
		t.Errorf("new target → action %q, want \"Yes, create & extract\"", rc.Actions[0].Label)
	}
	if !strings.Contains(rc.Description, "new worktree") {
		t.Errorf("new target recap should mention the new worktree, got:\n%s", rc.Description)
	}
}
