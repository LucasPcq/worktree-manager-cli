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

	tgt, err := resolveTarget(targetParams{
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

	tgt, err := resolveTarget(targetParams{
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

	_, err := resolveTarget(targetParams{
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

	if _, err := resolveTarget(targetParams{
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

	tgt, err := resolveTarget(targetParams{
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

	tgt, err := resolveTarget(targetParams{
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

	var got runpicker.WorktreePickerParams
	stubPicker(t, func(params runpicker.WorktreePickerParams) (domain.GitWorktree, error) {
		got = params
		return params.Worktrees[len(params.Worktrees)-1], nil
	})

	tgt, err := resolveTarget(targetParams{
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
	if got.Current != infra.ResolvePath(projectDir) {
		t.Errorf("the picker opened on %q, want the current worktree %q", got.Current, projectDir)
	}
	if len(got.Worktrees) != 3 {
		t.Errorf("the picker was offered %d worktrees, want all 3", len(got.Worktrees))
	}
	if tgt.Dir != got.Worktrees[len(got.Worktrees)-1].Path {
		t.Error("the picked worktree is not what the command acts on")
	}
}

// The job has no safe default: without --job, a run that cannot ask names the
// flag rather than falling back to a picker.
func TestResolveJobRequiresTheFlagWithoutATerminal(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{apiJob, migrateJob}}

	_, err := resolveJob(jobParams{Config: cfg, Interactive: false})
	if !errors.Is(err, domain.ErrJobRequired) {
		t.Fatalf("err = %v, want ErrJobRequired", err)
	}
}

func TestResolveJobRejectsAnUndeclaredName(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{apiJob}}

	_, err := resolveJob(jobParams{Name: "ghost", Config: cfg, Interactive: true})
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

func TestResolveJobPicksInteractively(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{apiJob, migrateJob}}

	previous := pickJob
	t.Cleanup(func() { pickJob = previous })
	pickJob = func(jobs []domain.JobConfig) (domain.JobConfig, error) {
		if len(jobs) != 2 {
			t.Errorf("the picker was offered %d jobs, want both", len(jobs))
		}
		return jobs[1], nil
	}

	job, err := resolveJob(jobParams{Config: cfg, Interactive: true})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if job.Name != migrateJob.Name {
		t.Errorf("resolved %q, want %q", job.Name, migrateJob.Name)
	}
}

func stubPicker(t *testing.T, fn func(runpicker.WorktreePickerParams) (domain.GitWorktree, error)) {
	t.Helper()
	previous := pickWorktree
	t.Cleanup(func() { pickWorktree = previous })
	pickWorktree = fn
}

func refusePicker(t *testing.T) {
	t.Helper()
	stubPicker(t, func(runpicker.WorktreePickerParams) (domain.GitWorktree, error) {
		t.Error("a picker was opened on a path that must never reach one")
		return domain.GitWorktree{}, nil
	})
}
