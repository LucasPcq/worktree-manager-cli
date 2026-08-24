package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func twoPorts() []PortEntry {
	return []PortEntry{
		{Job: "docker-compose", Name: "POSTGRES_PORT", Base: 5432},
		{Job: "web-dev", Name: "WEB_PORT", Base: 5173},
	}
}

func newPortList() PortListModel {
	return NewPortList(NewPortListParams{Title: "Ports", Entries: twoPorts()})
}

func portKey(m PortListModel, key string) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m
}

func portEnter(m PortListModel) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func portDown(m PortListModel) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	return m
}

func TestPortListStartsFromTheDetection(t *testing.T) {
	m := newPortList()
	got := m.Entries()
	if len(got) != 2 {
		t.Fatalf("expected the 2 detected ports, got %d", len(got))
	}
	if got[0].Base != 5432 {
		t.Errorf("base = %d, want the detected 5432", got[0].Base)
	}
}

func TestPortListEditsABase(t *testing.T) {
	// Enter sur une ligne l'édite — la même convention que HookListModel.
	m := portEnter(newPortList())
	m = portKey(m, "5555")
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5555 {
		t.Errorf("base = %d, want 5555", got)
	}
}

func TestPortListRefusesANonPort(t *testing.T) {
	// Une saisie invalide ne doit pas écraser une valeur détectée qui, elle,
	// marche : mieux vaut refuser que déclarer un port impossible.
	m := portEnter(newPortList())
	m = portKey(m, "99999")
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5432 {
		t.Errorf("base = %d, want the detected 5432 kept", got)
	}
	if m.Done() {
		t.Error("une saisie refusée ne valide pas l'étape")
	}
}

func TestPortListEditsTheRowUnderTheCursor(t *testing.T) {
	m := portDown(newPortList())
	m = portEnter(m)
	m = portKey(m, "5599")
	m = portEnter(m)

	got := m.Entries()
	if got[0].Base != 5432 {
		t.Errorf("la première ligne ne devait pas bouger, got %d", got[0].Base)
	}
	if got[1].Base != 5599 {
		t.Errorf("base = %d, want 5599", got[1].Base)
	}
}

func TestPortListKeepsTheDetectedValueOnAnEmptyEntry(t *testing.T) {
	m := portEnter(newPortList())
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5432 {
		t.Errorf("base = %d, want the detected 5432 kept", got)
	}
}

func TestPortListConfirmsOnTheDoneRow(t *testing.T) {
	m := newPortList()
	if m.Done() {
		t.Fatal("a fresh list is not done")
	}
	// Une ligne par port, puis la ligne « Done ».
	m = portDown(portDown(m))
	m = portEnter(m)
	if !m.Done() {
		t.Error("la ligne Done valide l'étape")
	}
}
