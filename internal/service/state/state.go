// Package state manages the global wtm state persisted at ~/.config/wtm/state.json.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// State represents the current wtm global state.
type State struct {
	ActiveWorktree     string `json:"active_worktree"`
	ActiveWorktreePath string `json:"active_worktree_path"`
	FocusedAt          string `json:"focused_at"`
}

// Load reads the state from ~/.config/wtm/state.json.
// Returns a zero-value State if the file does not exist.
func Load() (State, error) {
	return LoadFrom(statePath())
}

// Save writes the state to ~/.config/wtm/state.json.
func Save(s State) error {
	return SaveTo(statePath(), s)
}

// Clear empties the state file.
func Clear() error {
	return Save(State{})
}

// LoadFrom reads the state from a specific path (for testing).
func LoadFrom(path string) (State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}

	return s, nil
}

// SaveTo writes the state to a specific path (for testing).
func SaveTo(path string, s State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	return nil
}

func statePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, domain.GlobalConfigDir, domain.StateFileName)
}
