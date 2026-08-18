package flow

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These tests cover the create recap and the session composition: what the user
// reads before authorizing a creation, and which questions the flow declares.

// newFlow builds a create flow over a directory with no git repository, so branch
// divergence resolves as unknown and the tests stay hermetic.
func newFlow(t *testing.T, request CreateRequest, target func(string) domain.BranchTarget) *createFlow {
	t.Helper()
	config := domain.Config{}
	config.Project.Env.Strategy = domain.EnvStrategyExample
	if target == nil {
		target = func(string) domain.BranchTarget { return domain.BranchTarget{} }
	}
	return &createFlow{
		ctx:      Context{ProjectDir: t.TempDir(), Config: config},
		request:  request,
		prompter: Unattended{},
		target:   target,
	}
}

func existing(branchName string) func(string) domain.BranchTarget {
	return func(name string) domain.BranchTarget {
		if name == branchName {
			return domain.BranchTarget{State: domain.BranchTargetExisting}
		}
		return domain.BranchTarget{}
	}
}

// TestRecapKeepsEveryLineWhateverAnsweredIt: a flag resolves a step instead of
// asking it, and the recap must still name the value — otherwise a line silently
// disappears for exactly the users who passed the most flags.
func TestRecapKeepsEveryLineWhateverAnsweredIt(t *testing.T) {
	f := newFlow(t, CreateRequest{}, nil)
	answers := NewAnswers(map[string]string{
		KeyCreateBranch: "feat/x",
		KeyCreateSource: "main",
		KeyCreateEnv:    "example",
	})

	recap := f.recap(answers)
	for _, want := range []string{"Branch:  feat/x", "Source:  main", "Env:     example"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap %q should contain %q", recap, want)
		}
	}
}

func TestRecapNamesTheConfigDefaultEnv(t *testing.T) {
	f := newFlow(t, CreateRequest{}, nil)
	recap := f.recap(NewAnswers(map[string]string{KeyCreateBranch: "feat/x", KeyCreateSource: "main"}))
	if !strings.Contains(recap, "Env:     config default") {
		t.Errorf("recap %q should name the empty env choice", recap)
	}
}

// TestRecapCallsTheSourceAParentForAReusedBranch: a branch that already exists is
// checked out as-is, so the source is only what `wtm sync` will rebase onto.
func TestRecapCallsTheSourceAParentForAReusedBranch(t *testing.T) {
	f := newFlow(t, CreateRequest{}, existing("feat/x"))
	recap := f.recap(NewAnswers(map[string]string{
		KeyCreateBranch: "feat/x",
		KeyCreateSource: "main",
		KeyCreateEnv:    "example",
	}))

	if !strings.Contains(recap, "Parent:  main") {
		t.Errorf("recap %q should label the source as the recorded parent", recap)
	}
	if strings.Contains(recap, "Source:  ") {
		t.Errorf("recap %q must not present the parent as a start-point", recap)
	}
	if !strings.Contains(recap, domain.BranchReusedSuffix) {
		t.Errorf("recap %q should mark the branch as reused", recap)
	}
}

// TestRecapPutsTheFastForwardOnItsSubject: the annotation follows whichever branch
// is actually being moved — the source for a new branch, the branch itself when it
// is reused — so the recap can never claim to move a branch it leaves alone.
func TestRecapPutsTheFastForwardOnItsSubject(t *testing.T) {
	answers := NewAnswers(map[string]string{
		KeyCreateBranch:       "feat/x",
		KeyCreateSource:       "main",
		KeyCreateEnv:          "example",
		KeyCreateSourceUpdate: updateFastForward,
	})

	onSource := newFlow(t, CreateRequest{}, nil).recap(answers)
	if !strings.Contains(onSource, "Source:  main (fast-forward to origin)") {
		t.Errorf("recap %q should annotate the source line", onSource)
	}

	onBranch := newFlow(t, CreateRequest{}, existing("feat/x")).recap(answers)
	if !strings.Contains(onBranch, "fast-forward feat/x to origin") {
		t.Errorf("recap %q should carry its own update line for the reused branch", onBranch)
	}
	if strings.Contains(onBranch, "Parent:  main (fast-forward") {
		t.Errorf("recap %q must not annotate the parent it does not move", onBranch)
	}
}

// TestSessionAsksOnlyWhatIsMissing: a request value presets its step, so the wizard
// never re-asks something the command line already said.
func TestSessionAsksOnlyWhatIsMissing(t *testing.T) {
	full := newFlow(t, CreateRequest{Branch: "feat/x", From: "main", EnvFrom: "example"}, nil).session()
	for _, key := range []string{KeyCreateBranch, KeyCreateSource, KeyCreateEnv} {
		if _, preset := full.Presets.Get(key); !preset {
			t.Errorf("step %q should be answered by the request", key)
		}
	}

	bare := newFlow(t, CreateRequest{}, nil).session()
	for _, key := range []string{KeyCreateBranch, KeyCreateSource, KeyCreateEnv} {
		if _, preset := bare.Presets.Get(key); preset {
			t.Errorf("step %q should be left to be asked", key)
		}
	}
	// The steps themselves are always declared: the recap reads them all back.
	if len(bare.Steps) != 5 {
		t.Errorf("declared %d steps, want branch, source, env, source update and recap", len(bare.Steps))
	}
}

// TestBranchStepRefusesWithoutABranchName: a branch name has no safe default, so a
// run with nobody to ask must refuse rather than invent one.
func TestBranchStepRefusesWithoutABranchName(t *testing.T) {
	f := newFlow(t, CreateRequest{}, nil)

	if _, err := f.branchStep().Resolve(Answers{}); err == nil {
		t.Fatal("expected a refusal without a branch name")
	}
	if err := f.branchStep().Validate("   "); err == nil {
		t.Error("a blank branch name should be rejected as it is typed")
	}
	if err := f.branchStep().Validate("feat/x"); err != nil {
		t.Errorf("a real branch name should validate: %v", err)
	}
}

// TestSourceStepRefusesAGuessedParent: an existing branch was created outside wtm,
// so nothing says what it was branched off. Recording base_branch would be a guess
// that sync and tree then treat as fact.
func TestSourceStepRefusesAGuessedParent(t *testing.T) {
	f := newFlow(t, CreateRequest{}, existing("feat/x"))
	f.ctx.Config.Project.Worktrees.BaseBranch = "main"

	_, err := f.resolveSource(NewAnswers(map[string]string{KeyCreateBranch: "feat/x"}))
	if err == nil {
		t.Fatal("expected a refusal for a branch whose parent cannot be inferred")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Errorf("refusal %q should name the --%s flag", err, domain.FlagFrom)
	}
}

// TestSourceUpdateResolvesFromTheFlagOnly: --ff accepts the offer unattended, and
// its absence keeps the branch exactly where it is. --yes is never a mutation
// trigger of its own.
func TestSourceUpdateResolvesFromTheFlagOnly(t *testing.T) {
	withFlag := newFlow(t, CreateRequest{FastForward: true}, nil).sourceUpdateStep()
	answer, err := withFlag.Resolve(Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != updateFastForward {
		t.Errorf("answer = %q, want the fast-forward accepted", answer.Value)
	}

	without := newFlow(t, CreateRequest{}, nil).sourceUpdateStep()
	answer, err = without.Resolve(Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != updateKeep {
		t.Errorf("answer = %q, want the branch left as-is", answer.Value)
	}
}

func TestEnvSummaryNamesTheDefault(t *testing.T) {
	if got := envSummary(Answer{}); got != domain.EnvSummaryConfigDefault {
		t.Errorf("summary = %q, want the config default named", got)
	}
	if got := envSummary(Answer{Value: "main"}); got != "main" {
		t.Errorf("summary = %q, want the chosen strategy", got)
	}
}
