package state

import (
	"path/filepath"
	"testing"
)

func TestLoadFromMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ActiveWorktree != "" {
		t.Error("expected empty state for missing file")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wtm", "state.json")

	s := State{
		ActiveWorktree:     "feature/auth",
		ActiveWorktreePath: "/repo/.trees/feature-auth",
		FocusedAt:          "2026-04-02T10:00:00Z",
	}

	if err := SaveTo(path, s); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.ActiveWorktree != "feature/auth" {
		t.Errorf("expected feature/auth, got %s", loaded.ActiveWorktree)
	}
	if loaded.ActiveWorktreePath != "/repo/.trees/feature-auth" {
		t.Errorf("expected path, got %s", loaded.ActiveWorktreePath)
	}
	if loaded.FocusedAt != "2026-04-02T10:00:00Z" {
		t.Errorf("expected timestamp, got %s", loaded.FocusedAt)
	}
}

func TestClearState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wtm", "state.json")

	_ = SaveTo(path, State{
		ActiveWorktree: "feature/auth",
	})

	if err := SaveTo(path, State{}); err != nil {
		t.Fatalf("clear: %v", err)
	}

	loaded, _ := LoadFrom(path)
	if loaded.ActiveWorktree != "" {
		t.Error("expected empty state after clear")
	}
}
