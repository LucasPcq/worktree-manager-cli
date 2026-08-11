package extract

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func sourceStepWith(value string) components.Step {
	return components.Step{
		Name:  stepSource,
		Model: components.NewSelectList(components.NewSelectListParams{Items: []components.SelectItem{{Label: value, Value: value}}}),
	}
}

func TestSourceBranchFromPrev(t *testing.T) {
	if got := sourceBranchFromPrev([]components.Step{sourceStepWith("feat-a")}, "fixed"); got != "feat-a" {
		t.Errorf("picked source should win, got %q", got)
	}
	if got := sourceBranchFromPrev(nil, "fixed"); got != "fixed" {
		t.Errorf("no source step → fixed fallback, got %q", got)
	}
}

func TestFilesModel_Empty(t *testing.T) {
	m := filesModel("feat-a", nil)
	if got := m.View(); !strings.Contains(got, "No items") {
		t.Errorf("empty files model should render no items, got:\n%s", got)
	}
	// Enter on an empty source must be a no-op (Validate rejects an empty selection),
	// so the user is forced back to pick another worktree rather than proceeding.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.Done() {
		t.Error("empty files model must not complete on enter")
	}
}

func TestFilesModel_Populated(t *testing.T) {
	m := filesModel("feat-a", []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusUntracked}})
	if !strings.Contains(m.View(), "a.txt") {
		t.Errorf("populated files model should list the file, got:\n%s", m.View())
	}
}

func TestFilesModel_ShowsBothPathsOfARename(t *testing.T) {
	m := filesModel("feat-a", []domain.ExtractFile{
		{Path: "new.txt", OrigPath: "old.txt", Status: domain.ExtractStatusRenamed},
	})

	view := m.View()
	if !strings.Contains(view, "old.txt → new.txt") {
		t.Errorf("a rename should name both paths, got:\n%s", view)
	}
	if !strings.Contains(view, "ren") {
		t.Errorf("a rename should carry the ren tag, got:\n%s", view)
	}
}

func TestBuildCombinedRecap_ShowsSource(t *testing.T) {
	prev := []components.Step{sourceStepWith("feat-src"), targetStepWith("feat-a")}
	rc := buildCombinedRecap(prev, RunParams{})
	if !strings.Contains(rc.Description, "feat-src") {
		t.Errorf("recap should name the picked source, got:\n%s", rc.Description)
	}
}

func TestBuildCombinedRecap_FixedFlags(t *testing.T) {
	// Every part came from a flag (no file/target/mode steps): the recap still
	// names all of them.
	rc := buildCombinedRecap(nil, RunParams{
		SourceBranch: "feat-src",
		FixedFiles:   []string{"a.txt", "b.txt"},
		FixedTarget:  "feat-dst",
		FixedKeep:    true,
	})
	for _, want := range []string{"feat-src", "a.txt", "b.txt", "feat-dst", "copy"} {
		if !strings.Contains(rc.Description, want) {
			t.Errorf("recap missing %q, got:\n%s", want, rc.Description)
		}
	}
}

func TestBuildCombinedRecap_ShowsFixedSource(t *testing.T) {
	// When the source is given as an argument there is no source step; the recap
	// still names it from the fixed SourceBranch.
	prev := []components.Step{targetStepWith("feat-a")}
	rc := buildCombinedRecap(prev, RunParams{SourceBranch: "feat-fixed"})
	if !strings.Contains(rc.Description, "feat-fixed") {
		t.Errorf("recap should name the fixed source, got:\n%s", rc.Description)
	}
}

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
