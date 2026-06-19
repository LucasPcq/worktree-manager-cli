package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestBuildGlobalAnswers_Defaults(t *testing.T) {
	got, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Agent != domain.DefaultAgent {
		t.Errorf("Agent = %q, want default %q", got.Agent, domain.DefaultAgent)
	}
	if got.Shell != domain.DefaultShell {
		t.Errorf("Shell = %q, want default %q", got.Shell, domain.DefaultShell)
	}
}

func TestBuildGlobalAnswers_Flags(t *testing.T) {
	got, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{Agent: "cursor", Shell: "fish"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Agent != domain.AgentCursor || got.Shell != domain.ShellFish {
		t.Errorf("got %+v, want agent=cursor shell=fish", got)
	}
}

func TestBuildGlobalAnswers_Invalid(t *testing.T) {
	if _, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{Agent: "copilot"}); !errors.Is(err, domain.ErrInvalidAgentType) {
		t.Errorf("expected ErrInvalidAgentType, got %v", err)
	}
	if _, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{Shell: "powershell"}); !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func TestBuildProjectAnswers_FlagsWinOverDetection(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:     "develop",
		InstallCommand: "npm install",
		EnvFiles:       []string{".env"},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{
		BasePath:       "../wt",
		BaseBranch:     "main",
		EnvStrategy:    "parent",
		InstallCommand: "pnpm install",
	}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BasePath != "../wt" || got.BaseBranch != "main" {
		t.Errorf("flags did not win: %+v", got)
	}
	if got.EnvStrategy != domain.EnvStrategyParent || got.InstallCommand != "pnpm install" {
		t.Errorf("flags did not win: %+v", got)
	}
	if len(got.EnvCopyFiles) != 1 || got.EnvCopyFiles[0] != ".env" {
		t.Errorf("detected env files dropped: %+v", got.EnvCopyFiles)
	}
}

func TestBuildProjectAnswers_FallsBackToDetectionThenDefaults(t *testing.T) {
	detection := domain.InitDetectionResult{BaseBranch: "trunk", InstallCommand: "go mod download"}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BasePath != domain.DefaultBasePath {
		t.Errorf("BasePath = %q, want default", got.BasePath)
	}
	if got.BaseBranch != "trunk" {
		t.Errorf("BaseBranch = %q, want detected trunk", got.BaseBranch)
	}
	if got.EnvStrategy != domain.DefaultEnvStrategy {
		t.Errorf("EnvStrategy = %q, want default", got.EnvStrategy)
	}
	if got.InstallCommand != "go mod download" {
		t.Errorf("InstallCommand = %q, want detected", got.InstallCommand)
	}
}

func TestBuildProjectAnswers_NonInteractiveRequiresBaseBranch(t *testing.T) {
	_, err := rules.BuildProjectAnswers(rules.InitProjectFlags{NonInteractive: true}, domain.InitDetectionResult{})
	if err == nil {
		t.Fatal("expected error when base branch is unresolved in non-interactive mode")
	}
}

func TestBuildProjectAnswers_InvalidEnvStrategy(t *testing.T) {
	_, err := rules.BuildProjectAnswers(rules.InitProjectFlags{BaseBranch: "main", EnvStrategy: "nope"}, domain.InitDetectionResult{})
	if !errors.Is(err, domain.ErrInvalidEnvStrategy) {
		t.Errorf("expected ErrInvalidEnvStrategy, got %v", err)
	}
}

func TestBuildProjectAnswers_MonorepoToHooks(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:       "main",
		InstallCommand:   "pnpm install",
		MonorepoPackages: []string{"packages/a", "packages/b"},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.OnCreateExtra) != 2 {
		t.Fatalf("expected 2 on_create hooks, got %d", len(got.OnCreateExtra))
	}
	if got.OnCreateExtra[0].Cmd != "pnpm install" || got.OnCreateExtra[0].Cwd != "packages/a" {
		t.Errorf("unexpected hook: %+v", got.OnCreateExtra[0])
	}
}
