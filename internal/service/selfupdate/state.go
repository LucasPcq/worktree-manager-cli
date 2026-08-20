package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

func StatePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalStateFile), nil
}

// LoadState reads the update-check state. A missing or corrupt file means
// "never checked", never an error: the notifier must not fail a command.
func LoadState() domain.UpdateState {
	path, err := StatePath()
	if err != nil {
		return domain.UpdateState{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.UpdateState{}
	}

	var state domain.UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.UpdateState{}
	}

	return state
}

func SaveState(state domain.UpdateState) error {
	path, err := StatePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Written through a temp file: os.WriteFile truncates in place, so a process
	// exiting mid-write — which the refresh goroutine does by design — would
	// leave a partial file that costs an extra network call to recover from.
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+domain.GlobalStateFile+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}
