package flow

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These tests cover the clean confirmation body and its options — what the user
// reads before authorizing a removal, and the only place the force row is offered.

func TestDeleteRecapStatesWarningsAndTarget(t *testing.T) {
	recap := deleteRecap(domain.CleanCheckResult{
		Branch:          "feat",
		WorktreePath:    "/w/feat",
		IsDirty:         true,
		UnpushedCommits: 2,
		HasOpenPR:       true,
		PRUrl:           "http://pr",
	}, "")

	for _, want := range []string{"uncommitted changes", "2 commit(s)", "http://pr", "Will delete:", "/w/feat", "feat"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q:\n%s", want, recap)
		}
	}
}

func TestDeleteRecapCarriesTheReparentDecision(t *testing.T) {
	recap := deleteRecap(domain.CleanCheckResult{Branch: "feat", WorktreePath: "/w/feat"}, "Then leave 2 child worktree(s) orphaned.")
	if !strings.Contains(recap, "orphaned") {
		t.Errorf("recap should state what happens to the children:\n%s", recap)
	}
}

func TestDeleteOptionsOfferForceOnlyWhenUnsafe(t *testing.T) {
	safe := deleteOptions(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b"})
	if len(safe) != 1 || safe[0].Value != deleteYes {
		t.Fatalf("options = %+v, want the plain removal only", safe)
	}

	unsafe := deleteOptions(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b", IsDirty: true})
	var forced *Option
	for i := range unsafe {
		if unsafe[i].Value == deleteForce {
			forced = &unsafe[i]
		}
	}
	if forced == nil {
		t.Fatalf("options = %+v, want a force row for an unsafe worktree", unsafe)
	}
	if !forced.Danger {
		t.Error("the force row must read as destructive")
	}
}

func TestReparentProposalListsEveryMove(t *testing.T) {
	text := reparentProposal(domain.CleanReparentPlan{
		Grandparent: "gp",
		Children: []domain.ReparentResult{
			{Branch: "child", OldParent: "old", NewParent: "gp"},
			{Branch: "other", OldParent: "old", NewParent: "gp"},
		},
	})
	for _, want := range []string{"child", "other", "old", "gp"} {
		if !strings.Contains(text, want) {
			t.Errorf("proposal missing %q:\n%s", want, text)
		}
	}
}

// TestResolveDeleteForceSkipsTheCheck: --force is the safety axis, so it lifts the
// refusal without even running the check — which is what keeps a --yes --force run
// from touching the network.
func TestResolveDeleteForceSkipsTheCheck(t *testing.T) {
	f := &cleanFlow{
		request:   CleanRequest{Force: true},
		checks:    map[string]checkResult{},
		presenter: failingPresenter{t: t},
	}

	answer, err := f.resolveDelete(NewAnswers(map[string]string{KeyCleanWorktree: "feat"}))
	if err != nil {
		t.Fatalf("resolveDelete: %v", err)
	}
	if answer.Value != deleteYes {
		t.Errorf("answer = %q, want the removal authorized", answer.Value)
	}
}

// TestResolveDeleteKeepsSafetyWithoutForce: the confirmation axis does not lift
// safety — an unsafe worktree is refused, and the refusal names --force.
func TestResolveDeleteKeepsSafetyWithoutForce(t *testing.T) {
	f := &cleanFlow{
		checks: map[string]checkResult{
			"feat": {check: domain.CleanCheckResult{Branch: "feat", IsDirty: true}},
		},
	}

	_, err := f.resolveDelete(NewAnswers(map[string]string{KeyCleanWorktree: "feat"}))
	if err == nil {
		t.Fatal("expected an unsafe worktree to be refused")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal %q should direct to --force", err)
	}
}

// TestResolveDeleteAllowsASafeWorktree is the other half: nothing unsafe, nothing
// to refuse.
func TestResolveDeleteAllowsASafeWorktree(t *testing.T) {
	f := &cleanFlow{
		checks: map[string]checkResult{"feat": {check: domain.CleanCheckResult{Branch: "feat"}}},
	}

	answer, err := f.resolveDelete(NewAnswers(map[string]string{KeyCleanWorktree: "feat"}))
	if err != nil {
		t.Fatalf("resolveDelete: %v", err)
	}
	if answer.Value != deleteYes {
		t.Errorf("answer = %q, want the removal authorized", answer.Value)
	}
}

// TestPresetReparentAnswersTheStep: --reparent-children answers the reparent
// question through the presets, so the step is never asked and every reader — the
// recap line and the execution — sees the same answer.
func TestPresetReparentAnswersTheStep(t *testing.T) {
	forced := &cleanFlow{request: CleanRequest{ReparentChildren: true}}
	if got := forced.presetReparent(); got != reparentYes {
		t.Errorf("preset = %q, want the reparent authorized", got)
	}
	plain := &cleanFlow{}
	if got := plain.presetReparent(); got != "" {
		t.Errorf("preset = %q, want the step left to be answered", got)
	}
}

// failingPresenter fails the test if the flow shows anything at all.
type failingPresenter struct {
	t *testing.T
}

func (p failingPresenter) Stage(StageParams) error {
	p.t.Error("no progress should be shown")
	return nil
}
func (p failingPresenter) HookPhase(HookPhaseParams) error {
	p.t.Error("no hooks should run")
	return nil
}
func (p failingPresenter) Notice(Notice) { p.t.Error("no notice should be shown") }
func (p failingPresenter) Status(Notice) { p.t.Error("no status should be shown") }
func (p failingPresenter) Cleaned(CleanOutcome) error {
	p.t.Error("nothing should be concluded")
	return nil
}
