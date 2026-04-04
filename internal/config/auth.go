package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// authPath returns the full path to ~/.config/wtm/auth.json.
func authPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(configDir, domain.GlobalConfigDir, domain.AuthFileName), nil
}

// LoadAuth reads the stored GitHub auth token.
// Returns ErrAuthNotConfigured if no token file exists.
func LoadAuth() (domain.AuthToken, error) {
	path, err := authPath()
	if err != nil {
		return domain.AuthToken{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.AuthToken{}, domain.ErrAuthNotConfigured
	}
	if err != nil {
		return domain.AuthToken{}, fmt.Errorf("read %s: %w", path, err)
	}

	var token domain.AuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return domain.AuthToken{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return token, nil
}

// SaveAuth writes the GitHub auth token to ~/.config/wtm/auth.json with 0600 permissions.
func SaveAuth(token domain.AuthToken) error {
	path, err := authPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// DeleteAuth removes the stored GitHub auth token.
func DeleteAuth() error {
	path, err := authPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}

	return nil
}
