package sync

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// recordingPresenter captures the three moments in the order they fall: that
// order is the CLI output order the migration must preserve.
type recordingPresenter struct {
	order []string
}

func (p *recordingPresenter) Stage(params flow.StageParams) error {
	p.order = append(p.order, "stage:"+params.Message)
	return params.Work()
}

func (p *recordingPresenter) HookPhase(flow.HookPhaseParams) error { return nil }

func (p *recordingPresenter) Notice(notice flow.Notice) {
	p.order = append(p.order, "notice:"+notice.Text)
}

func (p *recordingPresenter) Status(flow.Notice) {}

func (p *recordingPresenter) Planned(domain.SyncPlan) { p.order = append(p.order, "planned") }

func (p *recordingPresenter) Rebased(domain.SyncResult) { p.order = append(p.order, "rebased") }

func (p *recordingPresenter) Synced(Outcome) error {
	p.order = append(p.order, "synced")
	return nil
}

// pushPrompter captures the push question instead of asking it. The cascade
// itself needs a repository, so it is characterized at the CLI level; what this
// package owns is how the question is put and when it is put at all.
type pushPrompter struct {
	interactive bool
	answer      bool
	asked       int
	params      flow.ConfirmParams
}

func (p *pushPrompter) Ask(flow.Session) (flow.Answers, error) { return flow.Answers{}, nil }

func (p *pushPrompter) Confirm(params flow.ConfirmParams) (bool, error) {
	p.asked++
	p.params = params
	return p.answer, nil
}

func (p *pushPrompter) Interactive() bool { return p.interactive }

func pushableResult() domain.SyncResult {
	return domain.SyncResult{Steps: []domain.SyncStepResult{{Branch: "feat-a", PushPending: true}}}
}

func TestOperationBlocksTheWholeSurface(t *testing.T) {
	op := Operation()

	if op.Mode != flow.ModeBlocking {
		t.Fatalf("sync must hold the surface, got mode %v", op.Mode)
	}
	if op.TargetKey != "" {
		t.Fatalf("holding everything, sync needs no per-worktree lock, got %q", op.TargetKey)
	}
	if op.Kind != domain.OpKindSync {
		t.Fatalf("Operation kind = %q, want %q", op.Kind, domain.OpKindSync)
	}
}

// The push is a labelled choice, not a yes/no: each outcome names what it does,
// and the warning travels as Warning so every surface folds it in the same way.
func TestPushQuestionNamesBothOutcomes(t *testing.T) {
	prompter := &pushPrompter{interactive: true, answer: true}
	f := &syncFlow{prompter: prompter}

	if !f.shouldPush(pushableResult()) {
		t.Fatal("an interactive run that accepted must push")
	}
	if prompter.params.YesLabel != domain.SyncPushOption || prompter.params.NoLabel != domain.SyncKeepLocalOption {
		t.Fatalf("the two outcomes must be named, got %+v", prompter.params)
	}
	if prompter.params.Warning != domain.SyncPushWarning || prompter.params.Description != "" {
		t.Fatalf("the warning must travel as Warning, got %+v", prompter.params)
	}
	if prompter.params.DefaultYes {
		t.Fatal("force-pushing with lease stays opt-in: keeping local must lead")
	}
}

func TestPushIsNeverAskedWithoutATerminal(t *testing.T) {
	prompter := &pushPrompter{interactive: false, answer: true}
	f := &syncFlow{prompter: prompter}

	if f.shouldPush(pushableResult()) {
		t.Fatal("a run that cannot ask must not push: --push is how it opts in")
	}
	if prompter.asked != 0 {
		t.Fatalf("nothing to ask on, yet the question was put %d time(s)", prompter.asked)
	}
}

func TestPushFlagSkipsTheQuestion(t *testing.T) {
	prompter := &pushPrompter{interactive: true}
	f := &syncFlow{prompter: prompter, request: Request{Push: true}}

	if !f.shouldPush(pushableResult()) {
		t.Fatal("--push must push")
	}
	if prompter.asked != 0 {
		t.Fatalf("--push already answered the question, asked %d time(s)", prompter.asked)
	}
}

func TestNoPushWinsOverAnAnsweredQuestion(t *testing.T) {
	prompter := &pushPrompter{interactive: true, answer: true}
	f := &syncFlow{prompter: prompter, request: Request{NoPush: true}}

	if f.shouldPush(pushableResult()) {
		t.Fatal("--no-push must never push")
	}
	if prompter.asked != 0 {
		t.Fatalf("--no-push settles it without asking, asked %d time(s)", prompter.asked)
	}
}

// The parent step reads staleParents from Skip and again from Build on every
// rebuild; the narrowing replans the cascade, so it must be paid once per
// selection.
func TestStaleParentsAreMemoizedPerSelection(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	f.classified = []domain.ParentUpdate{{Branch: "main", Behind: 1}}
	f.stale = map[string][]domain.ParentUpdate{"feat-a": {{Branch: "memoized"}}}

	parents := f.staleParents(flow.Answers{}.WithValues(KeySelection, []string{"feat-a"}))

	if len(parents) != 1 || parents[0].Branch != "memoized" {
		t.Fatalf("a selection already inspected must not be inspected again, got %+v", parents)
	}
}

// The one selection that both empties the plan and excludes the base: it
// concludes before anything is staged, so no plan and no recap are shown.
func TestAnEmptyCascadeConcludesBeforeStagingAnything(t *testing.T) {
	presenter := &recordingPresenter{}

	outcome, err := Run(Params{
		Context:   flow.Context{ProjectDir: gittest.InitRepo(t), StateDir: t.TempDir()},
		Request:   Request{Branches: []string{"main"}, BaseBranch: "other-base"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !outcome.Empty {
		t.Fatalf("no rebase step and no base refresh is an empty cascade, got %+v", outcome)
	}
	if outcome.Result.BaseBranch != "other-base" {
		t.Fatalf("the conclusion must carry the base it was asked about, got %q", outcome.Result.BaseBranch)
	}
	if got := strings.Join(presenter.order, ","); got != "synced" {
		t.Fatalf("an empty cascade only concludes, got %q", got)
	}
}

// A run that cannot ask never saw the plan in a recap, so the plan comes first,
// then the rebase, then what it did, and only then the conclusion.
func TestAnUnattendedRunShowsThePlanBeforeItRebases(t *testing.T) {
	presenter := &recordingPresenter{}

	if _, err := Run(Params{
		Context:   flow.Context{ProjectDir: gittest.InitRepo(t), StateDir: t.TempDir()},
		Request:   Request{All: true, BaseBranch: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "planned,stage:" + domain.SyncRebasing + ",rebased,synced"
	if got := strings.Join(presenter.order, ","); got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

// A run that can ask saw the plan on the recap, so it must not be shown a second
// time; the parent scan stays before the session, where its inputs are needed.
func TestAnAskedRunNeverShowsThePlanTwice(t *testing.T) {
	presenter := &recordingPresenter{}
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"main"}},
		Answers: map[string]string{KeyConfirm: confirmSync},
	}

	if _, err := Run(Params{
		Context:   flow.Context{ProjectDir: gittest.InitRepo(t), StateDir: t.TempDir()},
		Request:   Request{All: true, BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "stage:" + domain.SyncParentScanning + ",stage:" + domain.SyncRebasing + ",rebased,synced"
	if got := strings.Join(presenter.order, ","); got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

// --all is an explicit selection: it must never fall into the refusal that names
// it, however few worktrees hang off the base.
func TestAllNeverTripsTheSelectionRefusal(t *testing.T) {
	_, err := testFlow(Request{All: true}, nil).selectionStep().Resolve(flow.Answers{})

	if err != nil {
		t.Fatalf("--all answers the selection, got: %v", err)
	}
}

// A run with no branch arguments resolves nothing: the selection stays open for
// the picker rather than becoming an empty fixed set.
func TestResolvedBranchesLeavesAnEmptyRequestAlone(t *testing.T) {
	branches, err := (&syncFlow{}).resolvedBranches()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if branches != nil {
		t.Fatalf("no argument resolves to no selection, got %+v", branches)
	}
}
