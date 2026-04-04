package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const fullToml = `
[worktrees]
base_path = ".trees"
base_branch = "develop"

[env]
strategy = "main"
copy_files = [".env", "apps/api/.env"]

[hooks]
on_create = [
  "pnpm install",
  { cmd = "pnpm install", cwd = "apps/api" },
]
on_focus = ["docker-compose up -d"]
on_blur  = ["docker-compose down --remove-orphans"]

[github]
auto_draft  = true
base_branch = "develop"

[agents]
default = "cursor"

[integrations]
vscode_project_manager = true
cursor_project_manager = false
`

const minimalToml = `
[worktrees]
`

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFullConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, domain.ProjectDirName), domain.ConfigFileName, fullToml)

	cfg, err := Load(LoadParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Worktrees.BaseBranch != "develop" {
		t.Errorf("expected base_branch=develop, got %s", cfg.Project.Worktrees.BaseBranch)
	}
	if cfg.Project.Env.Strategy != domain.EnvStrategyMain {
		t.Errorf("expected strategy=main, got %s", cfg.Project.Env.Strategy)
	}
	if len(cfg.Project.Env.CopyFiles) != 2 {
		t.Errorf("expected 2 copy_files, got %d", len(cfg.Project.Env.CopyFiles))
	}
	if cfg.Project.Github.AutoDraft != true {
		t.Error("expected auto_draft=true")
	}
	if cfg.Project.Agents.Default != domain.AgentCursor {
		t.Errorf("expected agent=cursor, got %s", cfg.Project.Agents.Default)
	}
	if cfg.Project.Integrations.VSCodeProjectManager != true {
		t.Error("expected vscode_project_manager=true")
	}
}

func TestLoadMinimalConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, domain.ProjectDirName), domain.ConfigFileName, minimalToml)

	cfg, err := Load(LoadParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Worktrees.BasePath != domain.DefaultBasePath {
		t.Errorf("expected default base_path=%s, got %s", domain.DefaultBasePath, cfg.Project.Worktrees.BasePath)
	}
	if cfg.Project.Worktrees.BaseBranch != domain.DefaultBaseBranch {
		t.Errorf("expected default base_branch=%s, got %s", domain.DefaultBaseBranch, cfg.Project.Worktrees.BaseBranch)
	}
	if cfg.Project.Env.Strategy != domain.DefaultEnvStrategy {
		t.Errorf("expected default strategy=%s, got %s", domain.DefaultEnvStrategy, cfg.Project.Env.Strategy)
	}
	if cfg.Project.Agents.Default != domain.DefaultAgent {
		t.Errorf("expected default agent=%s, got %s", domain.DefaultAgent, cfg.Project.Agents.Default)
	}
	if cfg.Global.Shell != domain.DefaultShell {
		t.Errorf("expected default shell=%s, got %s", domain.DefaultShell, cfg.Global.Shell)
	}
}

func TestLoadMissingProjectConfigReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(LoadParams{ProjectDir: dir})
	if !errors.Is(err, domain.ErrConfigNotFound) {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestHookCommandStringParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, domain.ProjectDirName), domain.ConfigFileName, `
[hooks]
on_create = ["pnpm install"]
`)

	cfg, err := Load(LoadParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Project.Hooks.OnCreate) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(cfg.Project.Hooks.OnCreate))
	}

	hook := cfg.Project.Hooks.OnCreate[0]
	if hook.Cmd != "pnpm install" {
		t.Errorf("expected cmd=pnpm install, got %s", hook.Cmd)
	}
	if hook.Cwd != "" {
		t.Errorf("expected empty cwd, got %s", hook.Cwd)
	}
}

func TestHookCommandObjectParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, domain.ProjectDirName), domain.ConfigFileName, `
[hooks]
on_create = [
  { cmd = "pnpm install", cwd = "apps/api" },
]
`)

	cfg, err := Load(LoadParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Project.Hooks.OnCreate) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(cfg.Project.Hooks.OnCreate))
	}

	hook := cfg.Project.Hooks.OnCreate[0]
	if hook.Cmd != "pnpm install" {
		t.Errorf("expected cmd=pnpm install, got %s", hook.Cmd)
	}
	if hook.Cwd != "apps/api" {
		t.Errorf("expected cwd=apps/api, got %s", hook.Cwd)
	}
}

func TestHookCommandMixedParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, domain.ProjectDirName), domain.ConfigFileName, fullToml)

	cfg, err := Load(LoadParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hooks := cfg.Project.Hooks.OnCreate
	if len(hooks) != 2 {
		t.Fatalf("expected 2 on_create hooks, got %d", len(hooks))
	}

	if hooks[0].Cmd != "pnpm install" || hooks[0].Cwd != "" {
		t.Errorf("hook[0]: expected {cmd:pnpm install, cwd:}, got {cmd:%s, cwd:%s}", hooks[0].Cmd, hooks[0].Cwd)
	}
	if hooks[1].Cmd != "pnpm install" || hooks[1].Cwd != "apps/api" {
		t.Errorf("hook[1]: expected {cmd:pnpm install, cwd:apps/api}, got {cmd:%s, cwd:%s}", hooks[1].Cmd, hooks[1].Cwd)
	}
}

func TestValidateInvalidEnvStrategy(t *testing.T) {
	cfg := validConfig()
	cfg.Project.Env.Strategy = "invalid"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidEnvStrategy) {
		t.Errorf("expected ErrInvalidEnvStrategy, got %v", err)
	}
}

func TestValidateInvalidShellType(t *testing.T) {
	cfg := validConfig()
	cfg.Global.Shell = "powershell"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func TestValidateInvalidAgentType(t *testing.T) {
	cfg := validConfig()
	cfg.Project.Agents.Default = "copilot"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidAgentType) {
		t.Errorf("expected ErrInvalidAgentType, got %v", err)
	}
}

func TestMergeGlobalAgentFallback(t *testing.T) {
	project := domain.ProjectConfig{}
	global := domain.GlobalConfig{
		Shell: domain.ShellBash,
		Agent: domain.AgentCursor,
	}

	cfg := merge(project, global)

	if cfg.Project.Agents.Default != domain.AgentCursor {
		t.Errorf("expected global agent to fill project default, got %s", cfg.Project.Agents.Default)
	}
}

func TestMergeProjectOverridesGlobal(t *testing.T) {
	project := domain.ProjectConfig{
		Agents: domain.AgentsConfig{Default: domain.AgentNone},
	}
	global := domain.GlobalConfig{
		Agent: domain.AgentCursor,
	}

	cfg := merge(project, global)

	if cfg.Project.Agents.Default != domain.AgentNone {
		t.Errorf("expected project agent to take priority, got %s", cfg.Project.Agents.Default)
	}
}

func validConfig() domain.Config {
	return domain.Config{
		Project: domain.ProjectConfig{
			Worktrees: domain.WorktreesConfig{
				BasePath:   domain.DefaultBasePath,
				BaseBranch: domain.DefaultBaseBranch,
			},
			Env: domain.EnvConfig{
				Strategy: domain.DefaultEnvStrategy,
			},
			Agents: domain.AgentsConfig{
				Default: domain.DefaultAgent,
			},
		},
		Global: domain.GlobalConfig{
			Shell: domain.DefaultShell,
			Agent: domain.AgentType(domain.DefaultAgent),
		},
	}
}
