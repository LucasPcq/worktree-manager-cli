package sync

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// testFlow installs a Prompter: session() reads Interactive() to resolve the
// parent question from the flags.
func testFlow(request Request, statuses []domain.WorktreeStatus) *syncFlow {
	return &syncFlow{request: request, statuses: statuses, prompter: flow.Unattended{}}
}

var stack = []domain.WorktreeStatus{
	{Branch: "main", IsParent: true},
	{Branch: "feat-a"},
	{Branch: "feat-b", IsDirty: true},
	{Branch: "feat-c", RebaseInProgress: true},
}

func TestSelectionOptionsTagWhatSyncWouldSkip(t *testing.T) {
	content, err := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build selection: %v", err)
	}

	byValue := map[string]flow.Option{}
	for _, option := range content.Options {
		byValue[option.Value] = option
	}
	if byValue["main"].Label != "main"+domain.SyncBaseSuffix {
		t.Fatalf("the base must be labelled, got %q", byValue["main"].Label)
	}
	if byValue["feat-b"].Tag != domain.SyncTagDirty {
		t.Fatalf("a dirty worktree must be tagged, got %+v", byValue["feat-b"])
	}
	if byValue["feat-c"].Tag != domain.SyncTagRebasing {
		t.Fatalf("a worktree mid-rebase must be tagged, got %+v", byValue["feat-c"])
	}
}

// The CLI never sends a Precheck: its picker opens empty, as it does today.
func TestSelectionStartsUncheckedWithoutPrecheck(t *testing.T) {
	content, _ := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})

	for _, option := range content.Options {
		if option.Selected {
			t.Fatalf("%s must not arrive checked without Precheck", option.Value)
		}
	}
}

func TestSelectionPrechecksWhatTheSurfaceAsked(t *testing.T) {
	content, _ := testFlow(Request{Precheck: []string{"feat-a", "feat-b"}}, stack).
		selectionStep().Build(flow.Answers{})

	checked := map[string]bool{}
	for _, option := range content.Options {
		checked[option.Value] = option.Selected
	}
	if !checked["feat-a"] || !checked["feat-b"] || checked["feat-c"] {
		t.Fatalf("Precheck must decide exactly what arrives checked, got %+v", checked)
	}
}

// The CLAUDE.md model for the --yes axis: never a fallback picker, an error
// naming the missing flag.
func TestSelectionResolveNamesAll(t *testing.T) {
	_, err := testFlow(Request{}, stack).selectionStep().Resolve(flow.Answers{})

	if err == nil {
		t.Fatal("an unattended run with no selection must refuse")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestConflictStepDefaultsToAborting(t *testing.T) {
	answer, err := testFlow(Request{}, stack).conflictStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if answer.Value != conflictNormal {
		t.Fatalf("the safe default aborts the rebase, got %q", answer.Value)
	}
}

func TestConflictStepFollowsTheFlagWhenPreset(t *testing.T) {
	session := testFlow(Request{KeepConflict: true}, stack).session()

	answer, _ := session.Presets.Get(KeyConflict)
	if answer.Value != conflictKeep {
		t.Fatalf("--keep-conflict must preset the step, got %q", answer.Value)
	}
}

func TestParentsStepIsSkippedWhenNothingIsStale(t *testing.T) {
	skip, _ := testFlow(Request{}, stack).parentsStep().Skip(flow.Answers{})

	if !skip {
		t.Fatal("with no stale parent the question must not be asked")
	}
}

func TestRecapDescribesThePlan(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	f.plan = domain.SyncPlan{
		BaseBranch:   "main",
		BaseTargeted: true,
		Steps:        []domain.SyncStep{{Branch: "feat-a", SourceBranch: "main"}},
	}

	content, err := f.confirmStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build recap: %v", err)
	}
	if !strings.Contains(content.Description, "feat-a") {
		t.Fatalf("the recap must carry the plan, got: %q", content.Description)
	}
}
