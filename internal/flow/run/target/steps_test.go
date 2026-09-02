package target_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestWorktreeStepIsNotAskedInASingleWorktreeRepository(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	skip, reason := step.Skip(flow.Answers{})
	if !skip {
		t.Fatal("the step was asked in a repository holding one worktree")
	}
	if reason != domain.RunWorktreeOnlyOne {
		t.Errorf("reason = %q, want %q", reason, domain.RunWorktreeOnlyOne)
	}
}

// The current worktree is the answer whenever nobody is asked: standing in a
// worktree is what designates it, which is why `run` needs no picker to have a
// non-interactive path.
func TestWorktreeStepResolvesToTheCurrentWorktree(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	answer, err := step.Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Spelled the way git spells it, whatever the caller passed: the answer is
	// the daemon's key for every job the run touches.
	if answer.Value != target.Root(repo) {
		t.Errorf("Resolve = %q, want the current worktree %q", answer.Value, target.Root(repo))
	}
}

func TestWorktreeStepOffersEveryWorktreeWithItsRunningJobs(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreeStep(target.WorktreeParams{
		ProjectDir: repo,
		Current:    repo,
		Running:    map[string]int{second: 2},
	})

	if skip, _ := step.Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although the repository holds two worktrees")
	}

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Start != target.Root(repo) {
		t.Errorf("Start = %q, want the current worktree %q", content.Start, target.Root(repo))
	}
	if len(content.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(content.Options))
	}

	badges := badgesByValue(content.Options)
	if got := badges[second]; got != "2 running" {
		t.Errorf("second worktree badges = %q, want its running count", got)
	}
	if got := badges[target.Root(repo)]; got != domain.RunWorktreeCurrent {
		t.Errorf("current worktree badges = %q, want %q", got, domain.RunWorktreeCurrent)
	}
}

// The answer is a path, because that is the daemon's key; the recap has to read
// back as the branch the user typed.
func TestWorktreeStepSummarizesAPathAsItsBranch(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	if got := step.Summarize(flow.Answer{Value: second}); got != "feature" {
		t.Errorf("Summarize = %q, want the branch name", got)
	}
}

func TestJobStepRefusesInsteadOfPickingWhenNobodyCanBeAsked(t *testing.T) {
	step := target.JobStep(target.JobParams{
		Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Flag: domain.FlagJob,
	})

	if step.Resolve != nil {
		t.Error("the job step resolves, so an unattended run would answer it silently")
	}
	if step.Flag != domain.FlagJob {
		t.Errorf("Flag = %q, want the step to name --%s when it refuses", step.Flag, domain.FlagJob)
	}
}

func TestJobStepOffersEachJobWithItsKind(t *testing.T) {
	step := target.JobStep(target.JobParams{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
	}})

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	badges := badgesByValue(content.Options)
	if badges["web"] != string(domain.JobKindService) || badges["migrate"] != string(domain.JobKindTask) {
		t.Errorf("badges = %v, want each job tagged with its kind", badges)
	}
}

func TestJobStepSaysSoWhenThereIsNothingToPick(t *testing.T) {
	step := target.JobStep(target.JobParams{})

	if _, err := step.Build(flow.Answers{}); !errors.Is(err, domain.ErrNoJobsDeclared) {
		t.Errorf("Build error = %v, want ErrNoJobsDeclared", err)
	}
}

// One profile — or none — is a question with a single answer, so it is never
// put to anyone.
func TestProfileStepIsNotAskedWithoutAChoice(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "default"}},
		Default:  "default",
	})

	skip, reason := step.Skip(flow.Answers{})
	if !skip || reason != domain.RunProfileNoChoice {
		t.Errorf("Skip = (%v, %q), want the step skipped", skip, reason)
	}
}

func TestProfileStepResolvesToTheDefaultProfile(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "default"}, {Name: "full"}},
		Default:  "default",
	})

	if skip, _ := step.Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although two profiles exist")
	}

	answer, err := step.Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != "default" {
		t.Errorf("Resolve = %q, want the default profile", answer.Value)
	}

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Start != "default" {
		t.Errorf("Start = %q, want the cursor on the default profile", content.Start)
	}
	if !strings.Contains(content.Options[0].Label, "default") {
		t.Errorf("first option = %q, want it to name the profile", content.Options[0].Label)
	}
}

func TestDeclaredJobRefusesANameRunTomlDoesNotHold(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	if _, err := target.DeclaredJob(cfg, "ghost"); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("err = %v, want ErrJobNotFound", err)
	}
	if _, err := target.DeclaredJob(cfg, ""); !errors.Is(err, domain.ErrJobRequired) {
		t.Errorf("err = %v, want ErrJobRequired", err)
	}
	if job, err := target.DeclaredJob(cfg, "web"); err != nil || job.Name != "web" {
		t.Errorf("DeclaredJob = (%v, %v), want the declaration", job, err)
	}
}

func repoWithSecondWorktree(t *testing.T) (repo, second string) {
	t.Helper()
	repo = gittest.InitRepo(t)
	second = filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)
	return repo, target.Root(second)
}

func badgesByValue(options []flow.Option) map[string]string {
	byValue := make(map[string]string, len(options))
	for _, option := range options {
		texts := make([]string, 0, len(option.Badges))
		for _, badge := range option.Badges {
			texts = append(texts, badge.Text)
		}
		byValue[option.Value] = strings.Join(texts, " ")
	}
	return byValue
}
