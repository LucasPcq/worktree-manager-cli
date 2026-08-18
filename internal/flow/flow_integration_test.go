package flow

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests drive the flows through a scripted Prompter — a host that answers
// from a script instead of a keyboard, exercising the same Skip and Build hooks the
// wizard does. They cover the interactive path, which the CLI-level tests cannot
// reach without a terminal.

// scriptedPrompter answers a session from a script, recording what it was asked and
// the content a real host would have rendered.
type scriptedPrompter struct {
	answers map[string]string
	asked   []string
	content map[string]StepContent
	abort   bool
	confirm bool
}

func (p *scriptedPrompter) Ask(session Session) (Answers, error) {
	if p.abort {
		return Answers{}, domain.ErrUserAborted
	}
	if p.content == nil {
		p.content = map[string]StepContent{}
	}
	answers := session.Presets
	for _, step := range session.Steps {
		if _, known := answers.Get(step.Key); known {
			continue
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(answers); skip {
				answers = answers.With(step.Key, Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		content := StepContent{Title: step.Title, Description: step.Description, Options: step.Options}
		switch {
		case step.Build != nil:
			built, err := step.Build(answers)
			if err != nil {
				return Answers{}, err
			}
			content = built
		case step.Load != nil:
			loaded, err := step.Load(answers)
			if err != nil {
				return Answers{}, err
			}
			content = loaded
		}
		p.content[step.Key] = content

		value, scripted := p.answers[step.Key]
		if !scripted {
			return Answers{}, fmt.Errorf("nothing scripted for step %q", step.Key)
		}
		p.asked = append(p.asked, step.Key)
		answers = answers.With(step.Key, Answer{Value: value, Asked: true})
	}
	return answers, nil
}

func (p *scriptedPrompter) Confirm(ConfirmParams) (bool, error) { return p.confirm, nil }
func (p *scriptedPrompter) Interactive() bool                   { return true }

// recorder is a Presenter that runs the work and keeps what was shown.
type recorder struct {
	stages  []string
	hooks   []string
	notices []Notice
	status  []Notice
	created *CreateOutcome
	cleaned *CleanOutcome
}

func (r *recorder) Stage(params StageParams) error {
	r.stages = append(r.stages, params.Message)
	return params.Work()
}

func (r *recorder) HookPhase(params HookPhaseParams) error {
	r.hooks = append(r.hooks, params.Title)
	var sink strings.Builder
	return params.Run(&sink)
}

func (r *recorder) Notice(notice Notice) { r.notices = append(r.notices, notice) }
func (r *recorder) Status(notice Notice) { r.status = append(r.status, notice) }

func (r *recorder) Created(outcome CreateOutcome) error {
	r.created = &outcome
	return nil
}

func (r *recorder) Cleaned(outcome CleanOutcome) error {
	r.cleaned = &outcome
	return nil
}

// testContext prepares a repository and the flow context for it.
func testContext(t *testing.T) Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

// TestCreateInteractiveAsksEveryQuestionThenCreates walks the whole interactive
// path: the questions asked, the recap shown, and the worktree that comes out.
func TestCreateInteractiveAsksEveryQuestionThenCreates(t *testing.T) {
	ctx := testContext(t)
	prompter := &scriptedPrompter{answers: map[string]string{
		KeyCreateBranch: "feat/w",
		KeyCreateSource: "main",
		KeyCreateEnv:    "",
		KeyCreateRecap:  createConfirm,
	}}
	presenter := &recorder{}

	outcome, err := Create(CreateParams{
		Context:   ctx,
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := []string{KeyCreateBranch, KeyCreateSource, KeyCreateEnv, KeyCreateRecap}
	if strings.Join(prompter.asked, ",") != strings.Join(want, ",") {
		t.Errorf("asked %v, want %v", prompter.asked, want)
	}
	// The source-update step had nothing to reconcile (no remote), so it was skipped.
	if _, asked := prompter.content[KeyCreateSourceUpdate]; asked {
		t.Error("the source-update step should skip itself when there is nothing to reconcile")
	}

	recap := prompter.content[KeyCreateRecap].Description
	for _, line := range []string{"Branch:  feat/w", "Source:  main", "Env:     config default"} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should contain %q", recap, line)
		}
	}
	if presenter.created == nil || presenter.created.Branch != "feat/w" {
		t.Fatalf("created = %+v, want the new worktree reported", presenter.created)
	}
	if outcome.Result.Metadata.SourceBranch != "main" {
		t.Errorf("source_branch = %q, want the answered source recorded", outcome.Result.Metadata.SourceBranch)
	}
	if _, statErr := os.Stat(outcome.Result.Path); statErr != nil {
		t.Errorf("worktree not on disk: %v", statErr)
	}
	if len(presenter.stages) != 1 {
		t.Errorf("stages = %v, want just the creation", presenter.stages)
	}
}

// TestCreateSkipsTheQuestionsTheRequestAnswers: flags remove questions without
// removing anything from the recap.
func TestCreateSkipsTheQuestionsTheRequestAnswers(t *testing.T) {
	ctx := testContext(t)
	prompter := &scriptedPrompter{answers: map[string]string{KeyCreateRecap: createConfirm}}

	if _, err := Create(CreateParams{
		Context:   ctx,
		Request:   CreateRequest{Branch: "feat/flagged", From: "main", EnvFrom: "example"},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if strings.Join(prompter.asked, ",") != KeyCreateRecap {
		t.Errorf("asked %v, want the recap alone", prompter.asked)
	}
	recap := prompter.content[KeyCreateRecap].Description
	for _, line := range []string{"Branch:  feat/flagged", "Source:  main", "Env:     example"} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should still contain %q", recap, line)
		}
	}
}

// TestCreateAbortedCreatesNothing: cancelling is a clean exit — one notice, no
// worktree, no error.
func TestCreateAbortedCreatesNothing(t *testing.T) {
	ctx := testContext(t)
	presenter := &recorder{}

	outcome, err := Create(CreateParams{
		Context:   ctx,
		Request:   CreateRequest{Branch: "feat/nope", From: "main"},
		Prompter:  &scriptedPrompter{abort: true},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted {
		t.Error("outcome should report the abort")
	}
	if len(presenter.notices) != 1 || presenter.notices[0].Text != domain.AbortedMessage {
		t.Errorf("notices = %+v, want a single %q", presenter.notices, domain.AbortedMessage)
	}
	if presenter.created != nil {
		t.Error("nothing should have been created")
	}
	if len(presenter.stages) != 0 {
		t.Errorf("stages = %v, want none", presenter.stages)
	}
}

// TestCreateRunsHooksAsTheirOwnPhase: the hooks stream into the sink the presenter
// provides, after the creation and under their own title.
func TestCreateRunsHooksAsTheirOwnPhase(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "echo hooked"}}
	presenter := &recorder{}

	if _, err := Create(CreateParams{
		Context: ctx,
		Request: CreateRequest{Branch: "feat/hooked", From: "main"},
		Prompter: &scriptedPrompter{answers: map[string]string{
			KeyCreateEnv:   "",
			KeyCreateRecap: createConfirm,
		}},
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(presenter.hooks) != 1 || presenter.hooks[0] != domain.HooksTitleOnCreate {
		t.Errorf("hook phases = %v, want one titled %q", presenter.hooks, domain.HooksTitleOnCreate)
	}
}

// TestCreateRefusesABranchHeldElsewhere fails before asking anything: an
// interactive run that could only end in refusal should not be started.
func TestCreateRefusesABranchHeldElsewhereBeforeAsking(t *testing.T) {
	ctx := testContext(t)
	gittest.CreateBranch(t, ctx.ProjectDir, "feat/taken")
	elsewhere := filepath.Join(t.TempDir(), "taken")
	run(t, ctx.ProjectDir, "worktree", "add", elsewhere, "feat/taken")

	prompter := &scriptedPrompter{}
	_, err := Create(CreateParams{
		Context:   ctx,
		Request:   CreateRequest{Branch: "feat/taken", From: "main"},
		Prompter:  prompter,
		Presenter: &recorder{},
	})
	if !errors.Is(err, domain.ErrWorktreeExists) {
		t.Fatalf("err = %v, want it to wrap ErrWorktreeExists", err)
	}
	if len(prompter.asked) != 0 {
		t.Errorf("asked %v, want nothing asked before the refusal", prompter.asked)
	}
}

// TestCleanInteractiveConfirmsThenRemoves walks clean's interactive path: the
// confirmation states what will go, and the removal follows.
func TestCleanInteractiveConfirmsThenRemoves(t *testing.T) {
	ctx := testContext(t)
	created := mustCreate(t, ctx, "feat/gone")

	prompter := &scriptedPrompter{answers: map[string]string{KeyCleanDelete: deleteYes}}
	presenter := &recorder{}

	outcome, err := Clean(CleanParams{
		Context:   ctx,
		Request:   CleanRequest{Branch: "feat/gone", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}

	recap := prompter.content[KeyCleanDelete].Description
	for _, line := range []string{"Will delete:", "feat/gone"} {
		if !strings.Contains(recap, line) {
			t.Errorf("confirmation %q should contain %q", recap, line)
		}
	}
	if outcome.AlreadyAbsent {
		t.Error("the worktree existed, so this is a removal, not a no-op")
	}
	if presenter.cleaned == nil || presenter.cleaned.Branch != "feat/gone" {
		t.Fatalf("cleaned = %+v, want the removal reported", presenter.cleaned)
	}
	if _, statErr := os.Stat(created); !os.IsNotExist(statErr) {
		t.Errorf("worktree still on disk: %v", statErr)
	}
}

// TestCleanInteractiveOffersForceOnlyWhenUnsafe: the confirmation is where the
// safety warnings live, and the force row appears only when there is something to
// force — the axes stay separate even inside the wizard.
func TestCleanInteractiveOffersForceOnlyWhenUnsafe(t *testing.T) {
	ctx := testContext(t)
	path := mustCreate(t, ctx, "feat/dirty")
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("dirty the worktree: %v", err)
	}

	prompter := &scriptedPrompter{answers: map[string]string{KeyCleanDelete: deleteForce}}
	if _, err := Clean(CleanParams{
		Context:   ctx,
		Request:   CleanRequest{Branch: "feat/dirty", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	content := prompter.content[KeyCleanDelete]
	if !strings.Contains(content.Description, "uncommitted changes") {
		t.Errorf("confirmation should warn about the dirty worktree:\n%s", content.Description)
	}
	var hasForce bool
	for _, option := range content.Options {
		if option.Value == deleteForce {
			hasForce = true
		}
	}
	if !hasForce {
		t.Errorf("options = %+v, want a force row for a dirty worktree", content.Options)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("the forced removal should have removed the dirty worktree")
	}
}

// TestCleanAbsentWorktreeConcludesWithoutAsking: removing what is already gone is a
// success, reported before any question.
func TestCleanAbsentWorktreeConcludesWithoutAsking(t *testing.T) {
	ctx := testContext(t)
	prompter := &scriptedPrompter{}
	presenter := &recorder{}

	outcome, err := Clean(CleanParams{
		Context:   ctx,
		Request:   CleanRequest{Branch: "feat/ghost", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("cleaning an absent worktree must succeed: %v", err)
	}
	if !outcome.AlreadyAbsent {
		t.Error("outcome should report the no-op")
	}
	if len(prompter.asked) != 0 {
		t.Errorf("asked %v, want nothing asked", prompter.asked)
	}
	if presenter.cleaned == nil || !presenter.cleaned.AlreadyAbsent {
		t.Errorf("cleaned = %+v, want the no-op reported", presenter.cleaned)
	}
}

// TestCleanAbortedRemovesNothing: cancelling the confirmation leaves the worktree.
func TestCleanAbortedRemovesNothing(t *testing.T) {
	ctx := testContext(t)
	path := mustCreate(t, ctx, "feat/keep")
	presenter := &recorder{}

	outcome, err := Clean(CleanParams{
		Context:   ctx,
		Request:   CleanRequest{Branch: "feat/keep", BaseBranch: "main"},
		Prompter:  &scriptedPrompter{abort: true},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted {
		t.Error("outcome should report the abort")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the worktree must survive a cancelled clean: %v", statErr)
	}
}

// mustCreate creates a worktree through the flow and returns its path.
func mustCreate(t *testing.T, ctx Context, branchName string) string {
	t.Helper()
	outcome, err := Create(CreateParams{
		Context:   ctx,
		Request:   CreateRequest{Branch: branchName, From: "main", EnvFrom: "example"},
		Prompter:  Unattended{},
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("create %s: %v", branchName, err)
	}
	return outcome.Result.Path
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}
