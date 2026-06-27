package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func pressKey(m MultiSelectModel, key string) MultiSelectModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated
}

func newTestMultiSelect() MultiSelectModel {
	return NewMultiSelect(NewMultiSelectParams{
		Items: []MultiSelectItem{
			{Label: "a", Value: "a"},
			{Label: "b", Value: "b"},
			{Label: "c", Value: "c"},
		},
	})
}

func TestMultiSelectToggleAllSelectsEverything(t *testing.T) {
	m := pressKey(newTestMultiSelect(), "a")

	if got := m.Values(); len(got) != 3 {
		t.Fatalf("expected all 3 selected, got %v", got)
	}
}

func TestMultiSelectToggleAllClearsWhenFull(t *testing.T) {
	m := pressKey(newTestMultiSelect(), "a") // select all
	m = pressKey(m, "a")                     // toggle back to none

	if got := m.Values(); len(got) != 0 {
		t.Fatalf("expected empty selection after second toggle, got %v", got)
	}
}

func TestMultiSelectToggleAllFromPartialSelectsAll(t *testing.T) {
	m := newTestMultiSelect()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // select first item only
	m = pressKey(m, "a")                            // partial → all

	if got := m.Values(); len(got) != 3 {
		t.Fatalf("expected all 3 selected from partial, got %v", got)
	}
}
