package fastforward

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// flowWith builds a flow whose checks are already memoized, so the steps can be
// exercised without a git repository behind them.
func flowWith(request Request, checks map[string]domain.FastForwardCheck) *fastForwardFlow {
	statuses := make([]domain.WorktreeStatus, 0, len(checks))
	for name, check := range checks {
		statuses = append(statuses, domain.WorktreeStatus{
			Branch:       name,
			OriginState:  check.State,
			OriginBehind: check.Behind,
			OriginAhead:  check.Ahead,
		})
	}
	return &fastForwardFlow{request: request, statuses: statuses, checks: checks}
}

func selectedAnswers(branches ...string) flow.Answers {
	return flow.NewAnswers(nil).WithValues(KeySelection, branches)
}

func TestConfirmResolveRefusesADirtyWorktreeNamingForce(t *testing.T) {
	f := flowWith(Request{Branches: []string{"feat"}}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Behind: 2, State: domain.DivergenceBehind, IsDirty: true},
	})

	_, err := f.resolveConfirm(selectedAnswers("feat"))
	if err == nil {
		t.Fatal("resolveConfirm returned no error, want a refusal")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagForce) {
		t.Fatalf("error = %q, want it to name --%s", err, domain.FlagForce)
	}
}

func TestConfirmResolveAcceptsDirtyUnderForce(t *testing.T) {
	f := flowWith(Request{Branches: []string{"feat"}, Force: true}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Behind: 2, State: domain.DivergenceBehind, IsDirty: true},
	})

	answer, err := f.resolveConfirm(selectedAnswers("feat"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer.Value != confirmForce {
		t.Fatalf("answer = %q, want %q", answer.Value, confirmForce)
	}
}

func TestConfirmResolveAcceptsACleanBehindBranch(t *testing.T) {
	f := flowWith(Request{Branches: []string{"feat"}}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Behind: 2, State: domain.DivergenceBehind},
	})

	answer, err := f.resolveConfirm(selectedAnswers("feat"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer.Value != confirmYes {
		t.Fatalf("answer = %q, want %q", answer.Value, confirmYes)
	}
}

// A diverged branch is a refusal, not a blocker: --force must not reach it, so
// the confirm step resolves cleanly and the run reports the divergence instead.
func TestConfirmResolveDoesNotTreatDivergenceAsABlocker(t *testing.T) {
	f := flowWith(Request{Branches: []string{"feat"}}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Ahead: 1, Behind: 2, State: domain.DivergenceDiverged, IsDirty: true},
	})

	answer, err := f.resolveConfirm(selectedAnswers("feat"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer.Value != confirmYes {
		t.Fatalf("answer = %q, want %q", answer.Value, confirmYes)
	}
}

func TestSelectionResolveNamesTheAllFlag(t *testing.T) {
	f := flowWith(Request{}, map[string]domain.FastForwardCheck{})

	_, err := f.selectionStep().Resolve(flow.NewAnswers(nil))
	if err == nil {
		t.Fatal("Resolve returned no error, want a refusal naming --all")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("error = %q, want it to name --%s", err, domain.FlagAll)
	}
}

func TestBranchArgsPresetTheSelection(t *testing.T) {
	f := flowWith(Request{Branches: []string{"feat"}}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind},
	})
	f.selection = []string{"feat"}

	presets := f.session().Presets
	if got := presets.Values(KeySelection); len(got) != 1 || got[0] != "feat" {
		t.Fatalf("preset selection = %v, want [feat]", got)
	}
}

func TestPrecheckLeavesTheSelectionUnpreset(t *testing.T) {
	f := flowWith(Request{Precheck: []string{"feat"}}, map[string]domain.FastForwardCheck{
		"feat": {Branch: "feat", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind},
	})

	if got := f.session().Presets.Values(KeySelection); len(got) != 0 {
		t.Fatalf("preset selection = %v, want none: a precheck is offered, not decided", got)
	}
	options := f.selectionOptions()
	if len(options) != 1 || !options[0].Selected {
		t.Fatalf("options = %+v, want feat arriving checked", options)
	}
}

func TestRecapNamesEveryBranchIncludingTheRefusedOnes(t *testing.T) {
	body := recap([]domain.FastForwardCheck{
		{Branch: "feat", HasUpstream: true, Behind: 3, State: domain.DivergenceBehind},
		{Branch: "old", HasUpstream: true, Ahead: 1, Behind: 1, State: domain.DivergenceDiverged},
		{Branch: "done", HasUpstream: true, State: domain.DivergenceUpToDate},
	})
	for _, want := range []string{"feat", "old", "done", "wtm sync"} {
		if !strings.Contains(body, want) {
			t.Fatalf("recap = %q, want it to contain %q", body, want)
		}
	}
}

func TestBlockersAreNamedPerBranch(t *testing.T) {
	blockers := blockersOf([]domain.FastForwardCheck{
		{Branch: "a", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind, IsDirty: true},
		{Branch: "b", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind, IsDirty: true},
	})
	if len(blockers) != 2 {
		t.Fatalf("blockers = %d, want 2", len(blockers))
	}
	if blockers[0].Key == blockers[1].Key {
		t.Fatalf("blockers share the key %q; each must be liftable on its own", blockers[0].Key)
	}
}

// The danger option only exists when something stands in the way: offering
// "anyway" over a clean selection names a risk that is not there.
func TestConfirmOffersTheDangerOptionOnlyWhenBlocked(t *testing.T) {
	clean := confirmOptions([]domain.FastForwardCheck{
		{Branch: "feat", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind},
	})
	if len(clean) != 1 {
		t.Fatalf("options = %+v, want just the plain confirmation", clean)
	}

	blocked := confirmOptions([]domain.FastForwardCheck{
		{Branch: "feat", HasUpstream: true, Behind: 1, State: domain.DivergenceBehind, IsDirty: true},
	})
	if len(blocked) != 3 || blocked[2].Value != confirmForce || !blocked[2].Danger {
		t.Fatalf("options = %+v, want a danger force option", blocked)
	}
}

func TestOperationHoldsTheWholeSurface(t *testing.T) {
	op := Operation()
	if op.Kind != domain.OpKindFastForward {
		t.Fatalf("kind = %q, want %q", op.Kind, domain.OpKindFastForward)
	}
	if op.Mode != flow.ModeBlocking {
		t.Fatalf("mode = %v, want ModeBlocking", op.Mode)
	}
}
