package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newAutoSkipWizard(skipMiddle *bool) WizardModel {
	steps := []Step{
		{Name: "a", Model: NewTextInput(NewTextInputParams{Title: "a"})},
		{
			Name:     "b",
			Model:    NewTextInput(NewTextInputParams{Title: "b"}),
			AutoSkip: func(WizardModel) bool { return *skipMiddle },
		},
		{Name: "c", Model: NewTextInput(NewTextInputParams{Title: "c"})},
	}
	return NewWizard(steps)
}

func updateWizard(m WizardModel, msg tea.Msg) WizardModel {
	model, _ := m.Update(msg)
	wm, _ := model.(WizardModel)
	return wm
}

func TestWizardAutoSkipsStepAndMarksIt(t *testing.T) {
	skip := true
	m := newAutoSkipWizard(&skip)

	m = updateWizard(m, key(tea.KeyEnter)) // confirm step a → advance past auto-skipped b

	if m.current != 2 {
		t.Fatalf("expected to land on step c (2), got %d", m.current)
	}
	if !m.Skipped(1) {
		t.Error("expected auto-skipped step b to be marked skipped")
	}
}

func TestWizardShowsStepWhenAutoSkipFalse(t *testing.T) {
	skip := false
	m := newAutoSkipWizard(&skip)

	m = updateWizard(m, key(tea.KeyEnter)) // confirm step a → b is relevant, stop there

	if m.current != 1 {
		t.Fatalf("expected to land on step b (1), got %d", m.current)
	}
	if m.Skipped(1) {
		t.Error("step b should not be marked skipped when AutoSkip is false")
	}
}

func TestWizardGoBackHopsOverAutoSkipped(t *testing.T) {
	skip := true
	m := newAutoSkipWizard(&skip)

	m = updateWizard(m, key(tea.KeyEnter)) // a → c (b auto-skipped)
	if m.current != 2 || !m.Skipped(1) {
		t.Fatalf("precondition failed: current=%d skipped(1)=%v", m.current, m.Skipped(1))
	}

	m = updateWizard(m, key(tea.KeyEsc)) // back: hop over b → land on a

	if m.current != 0 {
		t.Fatalf("expected to hop back to step a (0), got %d", m.current)
	}
	if m.Skipped(1) {
		t.Error("hopping back should clear the auto-skip flag so it re-evaluates")
	}
}

func TestWizardFiresOnEnterWhenAdvancing(t *testing.T) {
	entered := 0
	steps := []Step{
		{Name: "a", Model: NewTextInput(NewTextInputParams{Title: "a"})},
		{
			Name:    "b",
			Model:   NewTextInput(NewTextInputParams{Title: "b"}),
			OnEnter: func([]Step) tea.Cmd { entered++; return nil },
		},
	}
	m := NewWizard(steps)

	m = updateWizard(m, key(tea.KeyEnter)) // confirm step a → advance into b
	if entered != 1 {
		t.Fatalf("OnEnter should fire once when advancing into step b, got %d", entered)
	}

	m = updateWizard(m, key(tea.KeyEsc)) // back to a (a has no OnEnter)
	if m.current != 0 {
		t.Fatalf("expected to be on step a after back, got %d", m.current)
	}
	if entered != 1 {
		t.Errorf("OnEnter should not fire on back navigation, got %d", entered)
	}
}

func TestWizardVisibleCounterExcludesSkipped(t *testing.T) {
	skip := true
	m := newAutoSkipWizard(&skip)

	m = updateWizard(m, key(tea.KeyEnter)) // a → c

	if got := m.visibleCount(); got != 2 {
		t.Errorf("visibleCount = %d, want 2 (a and c)", got)
	}
	if got := m.visiblePosition(); got != 2 {
		t.Errorf("visiblePosition = %d, want 2 (c is the 2nd visible step)", got)
	}
}
