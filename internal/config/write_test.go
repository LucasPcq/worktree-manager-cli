package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWriteProjectRendersValidTOML(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:       ".trees",
		BaseBranch:     "main",
		EnvCopyFiles:   []string{".env", "apps/api/.env"},
		EnvStrategy:    domain.EnvStrategyExample,
		InstallCommand: "pnpm install",
		Agent:          domain.AgentClaudeCode,
		AgentOverride:  false,
	}

	err := WriteProject(WriteProjectParams{
		ProjectDir: dir,
		Answers:    answers,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, domain.ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)

	// Verify it contains expected values
	if !strings.Contains(content, `base_path = ".trees"`) {
		t.Error("missing base_path")
	}
	if !strings.Contains(content, `strategy = "example"`) {
		t.Error("missing strategy")
	}
	if !strings.Contains(content, `"pnpm install"`) {
		t.Error("missing install command in hooks")
	}

	// Verify it parses as valid TOML
	var raw map[string]interface{}
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteProjectWithAgentOverride(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:      ".trees",
		BaseBranch:    "develop",
		EnvStrategy:   domain.EnvStrategyMain,
		Agent:         domain.AgentCursor,
		AgentOverride: true,
	}

	err := WriteProject(WriteProjectParams{
		ProjectDir: dir,
		Answers:    answers,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	if !strings.Contains(content, `default = "cursor"`) {
		t.Error("expected agent override to be written")
	}
}

func TestWriteProjectEmptyEnvAndHooks(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
	}

	err := WriteProject(WriteProjectParams{
		ProjectDir: dir,
		Answers:    answers,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	// Should still be valid TOML
	var raw map[string]interface{}
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteGlobalTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wtm", "config.toml")

	err := WriteGlobalTo(path, domain.InitGlobalAnswers{
		Agent: domain.AgentClaudeCode,
		Shell: domain.ShellZsh,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `shell = "zsh"`) {
		t.Error("missing shell")
	}
	if !strings.Contains(content, `agent = "claude-code"`) {
		t.Error("missing agent")
	}

	// Verify it parses as valid TOML
	var cfg domain.GlobalConfig
	if _, err := toml.Decode(content, &cfg); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
	if cfg.Shell != domain.ShellZsh {
		t.Errorf("expected shell=zsh, got %s", cfg.Shell)
	}
}

func TestWriteProjectRoundTrip(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:       ".trees",
		BaseBranch:     "develop",
		EnvCopyFiles:   []string{".env", "apps/web/.env"},
		EnvStrategy:    domain.EnvStrategyParent,
		InstallCommand: "yarn install",
		Agent:          domain.AgentNone,
		AgentOverride:  true,
	}

	err := WriteProject(WriteProjectParams{
		ProjectDir: dir,
		Answers:    answers,
	})
	if err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	// Load it back using the existing config loader
	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}

	if cfg.Worktrees.BasePath != ".trees" {
		t.Errorf("round-trip base_path: got %s", cfg.Worktrees.BasePath)
	}
	if cfg.Worktrees.BaseBranch != "develop" {
		t.Errorf("round-trip base_branch: got %s", cfg.Worktrees.BaseBranch)
	}
	if cfg.Env.Strategy != domain.EnvStrategyParent {
		t.Errorf("round-trip strategy: got %s", cfg.Env.Strategy)
	}
	if len(cfg.Env.CopyFiles) != 2 {
		t.Errorf("round-trip copy_files: got %d", len(cfg.Env.CopyFiles))
	}
	if len(cfg.Hooks.OnCreate) != 1 || cfg.Hooks.OnCreate[0].Cmd != "yarn install" {
		t.Errorf("round-trip on_create: got %v", cfg.Hooks.OnCreate)
	}
	if cfg.Agents.Default != domain.AgentNone {
		t.Errorf("round-trip agent: got %s", cfg.Agents.Default)
	}
}
