package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

const (
	deleteYes   = "yes"
	deleteForce = "force"
	reparentYes = "reparent"
	reparentNo  = "orphan"
)

// deleteSession mirrors what internal/flow/clean declares: the worktree comes
// preset from the selected row, the reparent decision and the confirmation are
// what the dashboard has to ask.
func deleteSession(blockers ...flow.Blocker) flow.Session {
	options := []flow.Option{{Label: "Yes, delete", Value: deleteYes}}
	if len(blockers) > 0 {
		options = append(options,
			flow.Option{Separator: true},
			flow.Option{Label: "Yes, force delete", Value: deleteForce, Danger: true},
		)
	}

	stated := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		stated = append(stated, blocker.Label)
	}

	return flow.Session{
		Presets: flow.NewAnswers(map[string]string{"worktree": "feat"}),
		Steps: []flow.Step{
			{Kind: flow.StepSelect, Key: "worktree", Label: "Worktree"},
			{
				Kind: flow.StepSelect, Key: "reparent", Title: "Reparent children",
				Description: "2 children point at feat",
				Options: []flow.Option{
					{Label: "Reparent onto main (2)", Value: reparentYes},
					{Separator: true},
					{Label: "Leave orphaned", Value: reparentNo},
				},
				Resolve: func(flow.Answers) (flow.Answer, error) {
					return flow.Answer{Value: reparentNo}, nil
				},
			},
			{
				Kind: flow.StepRecap, Key: "delete", Title: domain.CleanDeleteTitle,
				Options: options,
				Build: func(answers flow.Answers) (flow.StepContent, error) {
					return flow.StepContent{
						Description: strings.Join(append(stated, "", "Will delete: feat",
							"reparent decision: "+answers.Value("reparent")), "\n"),
						Options:  options,
						Blockers: blockers,
					}, nil
				},
			},
		},
	}
}

func openDeleteForm(t *testing.T, session flow.Session) (Model, chan promptReply) {
	t.Helper()
	model := newTestModel(t, testWidth, testHeight, "feat")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{
		title:   domain.DashboardDeleteTitle,
		shape:   modalForm,
		session: session,
		reply:   reply,
	})
	if len(model.modal.rows) == 0 {
		t.Fatal("the form must be built before it is shown")
	}
	return model, reply
}

func rowIndex(t *testing.T, model Model, match func(formRow) bool) int {
	t.Helper()
	for index, row := range model.modal.rows {
		if match(row) {
			return index
		}
	}
	t.Fatal("no such row in the form")
	return -1
}

func confirmIndex(t *testing.T, model Model) int {
	t.Helper()
	return rowIndex(t, model, func(row formRow) bool { return row.kind == formButton && row.confirm })
}

func blockerIndexes(model Model) []int {
	var indexes []int
	for index, row := range model.modal.rows {
		if row.kind == formBlocker {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func dirtyAndUnpushed() []flow.Blocker {
	return []flow.Blocker{
		{Key: domain.CleanBlockerDirty, Label: domain.CleanWarnDirty},
		{Key: domain.CleanBlockerUnpushed, Label: "2 commit(s) not pushed to remote"},
	}
}

func TestDeleteFormStatesEveryRefusalAsItsOwnAcknowledgement(t *testing.T) {
	model, _ := openDeleteForm(t, deleteSession(dirtyAndUnpushed()...))

	blockers := blockerIndexes(model)
	if len(blockers) != 2 {
		t.Fatalf("%d acknowledgements, want one per refusal — never a single blanket one", len(blockers))
	}
	for _, blocker := range dirtyAndUnpushed() {
		if rowIndex(t, model, func(row formRow) bool {
			return row.kind == formBlocker && row.label == blocker.Label
		}) < 0 {
			t.Errorf("refusal %q is not stated on its own row", blocker.Label)
		}
	}

	body := strings.Join(model.modal.body(m0zones{}), "\n")
	if strings.Count(body, domain.CleanWarnDirty) != 1 {
		t.Errorf("the dirty refusal is stated twice:\n%s", body)
	}
	if !strings.Contains(body, domain.DashboardBlockersTitle) {
		t.Error("the form must say what the acknowledgements are for")
	}
}

func TestDeleteStaysInertUntilEveryRefusalIsLifted(t *testing.T) {
	model, reply := openDeleteForm(t, deleteSession(dirtyAndUnpushed()...))
	blockers := blockerIndexes(model)

	model = pressRow(t, model, confirmIndex(t, model))
	select {
	case answered := <-reply:
		t.Fatalf("the removal was confirmed with nothing lifted: %+v", answered)
	default:
	}

	model = pressRow(t, model, blockers[0])
	model = pressRow(t, model, confirmIndex(t, model))
	select {
	case answered := <-reply:
		t.Fatalf("one refusal lifted out of two was enough: %+v", answered)
	default:
	}

	model = pressRow(t, model, blockers[1])
	model = pressRow(t, model, confirmIndex(t, model))

	answered := waitReply(t, reply)
	if answered.err != nil {
		t.Fatalf("confirmed with %v", answered.err)
	}
	if got := answered.answers.Value("delete"); got != deleteForce {
		t.Errorf("delete = %q, want the refusals lifted explicitly, one by one", got)
	}
}

func TestASafeWorktreeIsDeletedWithoutLiftingAnything(t *testing.T) {
	model, reply := openDeleteForm(t, deleteSession())

	if len(blockerIndexes(model)) != 0 {
		t.Fatal("nothing refuses the removal, so there is nothing to acknowledge")
	}

	model = pressRow(t, model, confirmIndex(t, model))

	answered := waitReply(t, reply)
	if got := answered.answers.Value("delete"); got != deleteYes {
		t.Errorf("delete = %q, want the plain removal — force is never implied", got)
	}
}

func TestTheReparentDecisionIsTakenInTheSameModal(t *testing.T) {
	model, reply := openDeleteForm(t, deleteSession())

	if got := model.modal.answers.Value("reparent"); got != reparentNo {
		t.Fatalf("reparent starts at %q, want the answer the flow resolves to when nobody is asked", got)
	}

	model = pressRow(t, model, rowIndex(t, model, func(row formRow) bool {
		return row.kind == formChoice && row.value == reparentYes
	}))

	body := strings.Join(model.modal.body(m0zones{}), "\n")
	if !strings.Contains(body, "reparent decision: "+reparentYes) {
		t.Errorf("the recap must follow the choice taken beside it:\n%s", body)
	}

	model = pressRow(t, model, confirmIndex(t, model))

	answered := waitReply(t, reply)
	if got := answered.answers.Value("reparent"); got != reparentYes {
		t.Errorf("reparent = %q, want the choice made in the modal", got)
	}
	if got := answered.answers.Value("worktree"); got != "feat" {
		t.Errorf("worktree = %q, want the preset read back", got)
	}
}

func TestCancellingTheDeleteFormAbortsTheRun(t *testing.T) {
	model, reply := openDeleteForm(t, deleteSession(dirtyAndUnpushed()...))

	model = press(t, model, namedKey(tea.KeyEsc))

	answered := waitReply(t, reply)
	if !errors.Is(answered.err, domain.ErrUserAborted) {
		t.Errorf("err = %v, want the removal aborted", answered.err)
	}
	if model.modal.open {
		t.Error("a cancelled form closes its modal")
	}
}

// pressRow walks the focus onto a row with the arrow keys and activates it, the
// way the keyboard drives the form.
func pressRow(t *testing.T, model Model, index int) Model {
	t.Helper()
	for guard := 0; model.modal.focus != index && guard <= len(model.modal.rows); guard++ {
		delta := 1
		if model.modal.focus > index {
			delta = -1
		}
		before := model.modal.focus
		model = press(t, model, namedKey(arrowFor(delta)))
		if model.modal.focus == before {
			t.Fatalf("focus stuck at %d, cannot reach row %d", before, index)
		}
	}
	if model.modal.focus != index {
		t.Fatalf("focus = %d, want row %d", model.modal.focus, index)
	}
	return press(t, model, namedKey(tea.KeyEnter))
}

func arrowFor(delta int) tea.KeyType {
	if delta < 0 {
		return tea.KeyUp
	}
	return tea.KeyDown
}
