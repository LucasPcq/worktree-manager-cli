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

// ErrRunFileExists is returned by WriteRun when the target file already
// exists — callers decide whether to skip or surface the condition.
var ErrRunFileExists = errors.New("run file already exists")

// projectTemplateData is the unified view rendered by the config template.
// Both the init wizard answers and a full ProjectConfig (for re-init) convert
// to it, so the template emits every section's actual values.
type projectTemplateData struct {
	BasePath     string
	BaseBranch   string
	SkipEnv      bool
	EnvStrategy  string
	EnvCopyFiles []string
	SkipHooks    bool
	OnCreate     []domain.HookCommand
	SkipClean    bool
	OnClean      []domain.HookCommand
}

// WriteProjectParams holds the inputs for writing a project config file.
type WriteProjectParams struct {
	StateDir string
	Answers  domain.InitProjectAnswers
}

// WriteProject renders the project config from init wizard answers and writes it
// to <state-dir>/config.toml.
func WriteProject(params WriteProjectParams) error {
	return renderProjectConfig(params.StateDir, answersToTemplate(params.Answers))
}

// WriteProjectConfigParams holds the inputs for rewriting config.toml from a
// full ProjectConfig (targeted re-init).
type WriteProjectConfigParams struct {
	StateDir string
	Config   domain.ProjectConfig
}

// WriteProjectConfig re-renders config.toml from a full ProjectConfig, preserving
// every section's current values (only manual comments are regenerated).
func WriteProjectConfig(params WriteProjectConfigParams) error {
	return renderProjectConfig(params.StateDir, configToTemplate(params.Config))
}

// renderProjectConfig renders the template data and writes config.toml.
func renderProjectConfig(stateDir string, data projectTemplateData) error {
	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", stateDir, err)
	}

	path := filepath.Join(stateDir, domain.ConfigFileName)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// answersToTemplate converts init wizard answers to template data.
func answersToTemplate(a domain.InitProjectAnswers) projectTemplateData {
	return projectTemplateData{
		BasePath:     a.BasePath,
		BaseBranch:   a.BaseBranch,
		SkipEnv:      a.SkipEnv,
		EnvStrategy:  string(a.EnvStrategy),
		EnvCopyFiles: a.EnvCopyFiles,
		SkipHooks:    a.SkipHooks,
		OnCreate:     a.OnCreate,
		SkipClean:    a.SkipClean,
		OnClean:      a.OnClean,
	}
}

// configToTemplate converts a full ProjectConfig to template data for re-init.
// An empty env strategy is rendered as a commented (skipped) section so the file
// stays valid.
func configToTemplate(c domain.ProjectConfig) projectTemplateData {
	return projectTemplateData{
		BasePath:     c.Worktrees.BasePath,
		BaseBranch:   c.Worktrees.BaseBranch,
		SkipEnv:      c.Env.Strategy == "",
		EnvStrategy:  string(c.Env.Strategy),
		EnvCopyFiles: c.Env.CopyFiles,
		SkipHooks:    false,
		OnCreate:     c.Hooks.OnCreate,
		SkipClean:    false,
		OnClean:      c.Hooks.OnClean,
	}
}

// WriteRunParams holds the inputs for writing a run config file.
type WriteRunParams struct {
	StateDir string
	Config   domain.RunConfig
	Force    bool // overwrite run.toml if it already exists
}

// WriteRun encodes cfg as TOML and writes it to <state-dir>/run.toml.
// Returns ErrRunFileExists if the file already exists and Force is false.
func WriteRun(params WriteRunParams) error {
	path := filepath.Join(params.StateDir, domain.RunFileName)

	if _, err := os.Stat(path); err == nil && !params.Force {
		return ErrRunFileExists
	}

	if err := os.MkdirAll(params.StateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", params.StateDir, err)
	}

	var buf bytes.Buffer
	buf.WriteString("#:schema ./schemas/run.schema.json\n\n")
	if err := toml.NewEncoder(&buf).Encode(params.Config); err != nil {
		return fmt.Errorf("encode run config: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// runTemplateContent is a fully-commented run.toml written when init detects or
// configures no services, so the user has a documented starting point to add
// jobs and profiles by hand later.
const runTemplateContent = `#:schema ./schemas/run.schema.json

# wtm — service & task definitions
# No services were configured during init. Uncomment and adapt the examples
# below, then manage them with ` + "`wtm run`" + `.

# A long-running service (started detached, stopped via its stop command):
# [[job]]
# name = "db"
# kind = "service"
# cmd  = "docker compose up -d"
# stop = "docker compose down --remove-orphans"
# cwd  = "."

# A one-shot task (must exit 0; output streamed live):
# [[job]]
# name = "migrate"
# kind = "task"
# cmd  = "pnpm run migrate"
# cwd  = "."

# A named group of jobs started together:
# [[profile]]
# name    = "default"
# jobs    = ["db"]
# default = true
`

// WriteRunTemplate writes a commented run.toml template to <state-dir>/run.toml.
// Returns ErrRunFileExists if the file already exists and Force is false.
func WriteRunTemplate(params WriteRunParams) error {
	path := filepath.Join(params.StateDir, domain.RunFileName)

	if _, err := os.Stat(path); err == nil && !params.Force {
		return ErrRunFileExists
	}

	if err := os.MkdirAll(params.StateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", params.StateDir, err)
	}

	if err := os.WriteFile(path, []byte(runTemplateContent), 0o644); err != nil {
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

	content := fmt.Sprintf("#:schema ./schemas/global.schema.json\n\nshell = %q\n", answers.Shell)

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

	content := fmt.Sprintf("#:schema ./schemas/global.schema.json\n\nshell = %q\n", answers.Shell)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
