package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ErrServicesFileExists is returned by WriteServices when the target file
// already exists — callers decide whether to skip or surface the condition.
var ErrServicesFileExists = errors.New("services file already exists")

// WriteProjectParams holds the inputs for writing a project config file.
type WriteProjectParams struct {
	ProjectDir string
	Answers    domain.InitProjectAnswers
}

// WriteProject renders the .wtm.toml template and writes it to the project directory.
func WriteProject(params WriteProjectParams) error {
	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, params.Answers); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	dir := filepath.Join(params.ProjectDir, domain.ProjectDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	path := filepath.Join(dir, domain.ConfigFileName)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// WriteServicesParams holds the inputs for writing a services config file.
type WriteServicesParams struct {
	ProjectDir string
	Config     domain.ServicesConfig
}

// WriteServices encodes cfg as TOML and writes it to .wtm/services.toml.
// Returns ErrServicesFileExists if the file already exists; the file is never
// overwritten.
func WriteServices(params WriteServicesParams) error {
	dir := filepath.Join(params.ProjectDir, domain.ProjectDirName)
	path := filepath.Join(dir, domain.ServicesFileName)

	if _, err := os.Stat(path); err == nil {
		return ErrServicesFileExists
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(params.Config); err != nil {
		return fmt.Errorf("encode services: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// WriteGlobal creates the global config directory and writes config.toml.
func WriteGlobal(answers domain.InitGlobalAnswers) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("user config dir: %w", err)
	}

	dir := filepath.Join(configDir, domain.GlobalConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content := fmt.Sprintf("shell = %q\nagent = %q\n", answers.Shell, answers.Agent)

	path := filepath.Join(dir, domain.GlobalConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// WriteGlobalTo writes the global config to a specific path (for testing).
func WriteGlobalTo(path string, answers domain.InitGlobalAnswers) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content := fmt.Sprintf("shell = %q\nagent = %q\n", answers.Shell, answers.Agent)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
