package dashboard

import (
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// The create command itself is not run here: startCreate hands the flow to a
// bubbletea command, and this asserts what the dashboard does before and around
// it. What the flow asks is covered by the modal tests.
func TestNewKeyStartsACreateRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := updateCmd(model, key(domain.KeyNew))

	if cmd == nil {
		t.Fatal("n must start the create flow")
	}
	if len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want the run recorded", model.ops.running)
	}
	if got := model.ops.running[0]; got.kind != domain.OpKindCreate || got.mode != flow.ModeBackground {
		t.Errorf("operation = %+v, want the mode create declares", got)
	}
	if !model.outputExpanded {
		t.Error("a run the user cannot watch is a run they cannot trust: the output panel must open")
	}
}

func TestFinishedRunReleasesItsSlotAndReportsAFailure(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))
	id := model.ops.running[0].id

	model = update(model, opDoneMsg{id: id, err: domain.ErrWorktreeExists})

	if len(model.ops.running) != 0 {
		t.Fatalf("running = %+v, want the finished run released", model.ops.running)
	}
	if len(model.outputLines) == 0 {
		t.Fatal("a failed run must say so in the output panel")
	}
}

func TestAbortedRunSaysNothingExtra(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))

	model = update(model, opDoneMsg{id: model.ops.running[0].id, err: domain.ErrUserAborted})

	if len(model.outputLines) != 0 {
		t.Errorf("output = %q, want nothing: the flow already noticed the abort", model.outputLines)
	}
}

// The hooks write from the flow's goroutine while the run is still going: the
// panel must fill as they print, not once they are done.
func TestHooksStreamIntoTheOutputPanelLineByLine(t *testing.T) {
	msgs := make(chan tea.Msg, 16)
	p := presenter{send: func(msg tea.Msg) { msgs <- msg }}

	err := p.HookPhase(flow.HookPhaseParams{
		Title: domain.HooksTitleOnCreate,
		Run: func(sink io.Writer) error {
			if _, writeErr := io.WriteString(sink, "installing\npar"); writeErr != nil {
				return writeErr
			}
			if len(msgs) != 2 {
				t.Errorf("%d lines posted mid-run, want the title and the first line already out", len(msgs))
			}
			_, writeErr := io.WriteString(sink, "tial tail")
			return writeErr
		},
	})
	if err != nil {
		t.Fatalf("hook phase: %v", err)
	}

	want := []string{domain.HooksTitleOnCreate, "installing", "partial tail"}
	if len(msgs) != len(want) {
		t.Fatalf("%d lines posted, want %d", len(msgs), len(want))
	}
	for _, expected := range want {
		line, ok := (<-msgs).(OutputLineMsg)
		if !ok {
			t.Fatalf("the hook sink posted something that is not an output line")
		}
		if line.Text != expected {
			t.Errorf("line = %q, want %q", line.Text, expected)
		}
	}
}

func TestASuccessfulCreateSelectsTheNewWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model, cmd := model.applyFlow(createdMsg{branch: "c"})
	if cmd == nil {
		t.Fatal("a finished create must refresh the list")
	}
	if model.cursor != 0 {
		t.Fatal("the selection only moves once the list holds the new worktree")
	}

	model = update(model, worktreesMsg{statuses: statuses("a", "b", "c"), parents: map[string]string{}})

	if model.cursor != 2 {
		t.Errorf("cursor = %d, want the new worktree selected", model.cursor)
	}
	selected, _ := model.selected()
	if selected.Branch != "c" {
		t.Errorf("selected %q, want the worktree that was just created", selected.Branch)
	}
}

func TestAFlowMessageIsDeliveredThroughTheChannelAndRearmsTheListener(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model.msgs <- OutputLineMsg{Text: "from the flow"}
	msg, ok := listenCmd(model.msgs)().(flowMsg)
	if !ok {
		t.Fatalf("listenCmd returned %T, want a flowMsg wrapping what was posted", msg)
	}

	model, cmd := updateCmd(model, msg)

	if len(model.outputLines) != 1 || model.outputLines[0] != "from the flow" {
		t.Errorf("output = %q, want the posted line", model.outputLines)
	}
	if cmd == nil {
		t.Error("handling a flow message must re-arm the listener, or the next one is never read")
	}
}
