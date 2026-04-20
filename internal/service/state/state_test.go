package state

import (
	"path/filepath"
	"testing"
)

func TestLoadFromMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	_, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wtm", "state.json")

	if err := SaveTo(path, State{}); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestClearState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wtm", "state.json")

	_ = SaveTo(path, State{})

	if err := SaveTo(path, State{}); err != nil {
		t.Fatalf("clear: %v", err)
	}

	_, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
}
