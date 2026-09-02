package run

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// setupWorktrees gives the repo two extra worktrees, so resolveTarget has
// something to match a query against and something to offer a picker.
func setupWorktrees(t *testing.T) (projectDir string) {
	t.Helper()
	dir := gittest.InitRepo(t)
	trees := filepath.Join(t.TempDir(), "trees")
	for _, branch := range []string{"feature/api-rewrite", "feature/web"} {
		gittest.Git(t, dir, "worktree", "add", "-b", branch, filepath.Join(trees, filepath.Base(branch)))
	}
	return dir
}

func TestResolveTargetExactBranchWins(t *testing.T) {
	projectDir := setupWorktrees(t)

	tgt, err := resolveInputs(inputsParams{
		Args:       []string{"feature/web"},
		Cwd:        projectDir,
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tgt.Branch != "feature/web" {
		t.Errorf("resolved %q, want feature/web", tgt.Branch)
	}
	if tgt.Dir == projectDir {
		t.Error("resolved to the current directory instead of the named worktree")
	}
}

func TestResolveTargetAcceptsAUniqueSubstring(t *testing.T) {
	projectDir := setupWorktrees(t)

	tgt, err := resolveInputs(inputsParams{
		Args:       []string{"rewrite"},
		Cwd:        projectDir,
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tgt.Branch != "feature/api-rewrite" {
		t.Errorf("resolved %q, want feature/api-rewrite", tgt.Branch)
	}
}

// An ambiguous query names the candidates rather than picking one or opening a
// picker: the caller has to be able to disambiguate without guessing.
func TestResolveTargetAmbiguousQueryNamesTheCandidates(t *testing.T) {
	projectDir := setupWorktrees(t)

	_, err := resolveInputs(inputsParams{
		Args:       []string{"feature"},
		Cwd:        projectDir,
		ProjectDir: projectDir,
	})
	if err == nil {
		t.Fatal("an ambiguous query resolved to a single worktree")
	}
	for _, branch := range []string{"feature/api-rewrite", "feature/web"} {
		if !strings.Contains(err.Error(), branch) {
			t.Errorf("error %q does not name %s", err, branch)
		}
	}
}

func TestResolveTargetUnknownQueryFails(t *testing.T) {
	projectDir := setupWorktrees(t)

	if _, err := resolveInputs(inputsParams{
		Args:       []string{"nope"},
		Cwd:        projectDir,
		ProjectDir: projectDir,
	}); err == nil {
		t.Fatal("an unknown worktree resolved instead of failing")
	}
}

// The worktree is a decision with a safe default: without a terminal, an omitted
// positional is the current directory and no picker is ever reached.
func TestResolveTargetFallsBackToTheCwdWithoutATerminal(t *testing.T) {
	projectDir := setupWorktrees(t)
	refusePicker(t)

	tgt, err := resolveInputs(inputsParams{
		Cwd:         projectDir,
		ProjectDir:  projectDir,
		Interactive: false,
		Pick:        true,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tgt.Dir != infra.ResolvePath(projectDir) {
		t.Errorf("resolved %q, want the current worktree %q", tgt.Dir, projectDir)
	}
}

// `run url` never opens a picker on any axis, so it keeps the cwd even when the
// run is otherwise interactive.
func TestResolveTargetKeepsTheCwdWhenPickingIsRefused(t *testing.T) {
	projectDir := setupWorktrees(t)
	refusePicker(t)

	tgt, err := resolveInputs(inputsParams{
		Cwd:         projectDir,
		ProjectDir:  projectDir,
		Interactive: true,
		Pick:        false,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tgt.Dir != infra.ResolvePath(projectDir) {
		t.Errorf("resolved %q, want the current worktree %q", tgt.Dir, projectDir)
	}
}

func TestResolveTargetOpensThePickerOnTheCurrentWorktree(t *testing.T) {
	projectDir := setupWorktrees(t)

	var got runpicker.TargetWizardParams
	stubWizard(t, func(params runpicker.TargetWizardParams) (runpicker.TargetWizardResult, error) {
		got = params
		last := params.Worktree.Worktrees[len(params.Worktree.Worktrees)-1]
		return runpicker.TargetWizardResult{WorktreePath: last.Path}, nil
	})

	tgt, err := resolveInputs(inputsParams{
		Cwd:         projectDir,
		ProjectDir:  projectDir,
		Interactive: true,
		Pick:        true,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// The picker starts on an item's Value, which is the path as git spells it —
	// /private/var where the cwd says /var on macOS.
	if got.Worktree.Current != infra.ResolvePath(projectDir) {
		t.Errorf("the form opened on %q, want the current worktree %q", got.Worktree.Current, projectDir)
	}
	if len(got.Worktree.Worktrees) != 3 {
		t.Errorf("the form was offered %d worktrees, want all 3", len(got.Worktree.Worktrees))
	}
	if tgt.Dir != got.Worktree.Worktrees[len(got.Worktree.Worktrees)-1].Path {
		t.Error("the picked worktree is not what the command acts on")
	}
}

// The job has no safe default: without --job, a run that cannot ask names the
// flag rather than falling back to a form.
func TestSecondAxisRequiresTheFlagWithoutATerminal(t *testing.T) {
	projectDir := setupWorktrees(t)
	refusePicker(t)

	_, err := resolveInputs(inputsParams{
		Cwd:        projectDir,
		ProjectDir: projectDir,
		Second:     secondAxis{Jobs: []domain.JobConfig{apiJob, migrateJob}, Required: true},
	})
	if !errors.Is(err, domain.ErrJobRequired) {
		t.Fatalf("err = %v, want ErrJobRequired", err)
	}
}

func TestDeclaredJobRejectsAnUndeclaredName(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{apiJob}}

	if _, err := declaredJob(cfg, "ghost"); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

// Both questions go into one form, which is what lets the reader step back from
// the second to the first instead of losing the run.
func TestBothQuestionsAreAskedInOneForm(t *testing.T) {
	projectDir := setupWorktrees(t)

	var got runpicker.TargetWizardParams
	stubWizard(t, func(params runpicker.TargetWizardParams) (runpicker.TargetWizardResult, error) {
		got = params
		return runpicker.TargetWizardResult{
			WorktreePath: params.Worktree.Worktrees[0].Path,
			Second:       migrateJob.Name,
		}, nil
	})

	resolved, err := resolveInputs(inputsParams{
		Cwd:         projectDir,
		ProjectDir:  projectDir,
		Interactive: true,
		Pick:        true,
		Second:      secondAxis{Jobs: []domain.JobConfig{apiJob, migrateJob}, Required: true},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Worktree == nil || got.Second == nil {
		t.Fatal("the two questions were not asked in the same form")
	}
	if resolved.Second != migrateJob.Name {
		t.Errorf("resolved %q, want %q", resolved.Second, migrateJob.Name)
	}
}

// A question the flags already answered is not a step: it costs no keystroke and
// does not appear in the breadcrumb.
func TestAnAnsweredQuestionIsNotAStep(t *testing.T) {
	projectDir := setupWorktrees(t)

	var got runpicker.TargetWizardParams
	stubWizard(t, func(params runpicker.TargetWizardParams) (runpicker.TargetWizardResult, error) {
		got = params
		return runpicker.TargetWizardResult{WorktreePath: params.Worktree.Worktrees[0].Path}, nil
	})

	resolved, err := resolveInputs(inputsParams{
		Cwd:         projectDir,
		ProjectDir:  projectDir,
		Interactive: true,
		Pick:        true,
		Second:      secondAxis{Given: apiJob.Name, Jobs: []domain.JobConfig{apiJob, migrateJob}, Required: true},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Second != nil {
		t.Error("a question answered by a flag was asked again")
	}
	if resolved.Second != apiJob.Name {
		t.Errorf("the flag's value was lost: got %q", resolved.Second)
	}
}

func stubWizard(t *testing.T, fn func(runpicker.TargetWizardParams) (runpicker.TargetWizardResult, error)) {
	t.Helper()
	previous := askWizard
	t.Cleanup(func() { askWizard = previous })
	askWizard = fn
}

func refusePicker(t *testing.T) {
	t.Helper()
	stubWizard(t, func(runpicker.TargetWizardParams) (runpicker.TargetWizardResult, error) {
		t.Error("a form was opened on a path that must never reach one")
		return runpicker.TargetWizardResult{}, nil
	})
}
