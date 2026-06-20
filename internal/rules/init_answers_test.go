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
	if got.EnvStrategy != domain.EnvStrategyParent {
		t.Errorf("flags did not win: %+v", got)
	}
	if len(got.OnCreate) == 0 || got.OnCreate[0].Cmd != "pnpm install" {
		t.Errorf("install flag should win in on_create: %+v", got.OnCreate)
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
	if len(got.OnCreate) == 0 || got.OnCreate[0].Cmd != "go mod download" {
		t.Errorf("on_create = %+v, want detected install", got.OnCreate)
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

func TestBuildProjectAnswers_SkipEnv(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch: "main",
		EnvFiles:   []string{".env", ".env.local"},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{SkipEnv: true}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SkipEnv {
		t.Error("SkipEnv = false, want true")
	}
	if got.EnvStrategy != "" {
		t.Errorf("EnvStrategy = %q, want empty when skipped", got.EnvStrategy)
	}
	if len(got.EnvCopyFiles) != 0 {
		t.Errorf("EnvCopyFiles = %v, want none when skipped", got.EnvCopyFiles)
	}
}

func TestBuildProjectAnswers_SkipEnvIgnoresInvalidStrategy(t *testing.T) {
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{
		BaseBranch:  "main",
		EnvStrategy: "nope",
		SkipEnv:     true,
	}, domain.InitDetectionResult{})
	if err != nil {
		t.Fatalf("skipping env must bypass strategy validation, got %v", err)
	}
	if !got.SkipEnv {
		t.Error("SkipEnv = false, want true")
	}
}

func TestBuildProjectAnswers_SkipHooks(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:       "main",
		InstallCommand:   "pnpm install",
		MonorepoPackages: []string{"packages/a"},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{SkipHooks: true}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SkipHooks {
		t.Error("SkipHooks = false, want true")
	}
	if len(got.OnCreate) != 0 {
		t.Errorf("OnCreate = %v, want none when skipped", got.OnCreate)
	}
}

func TestBuildProjectAnswers_SkipServices(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:         "main",
		DockerComposeFiles: []string{"docker-compose.yml"},
		DockerComposeCmd:   "docker compose",
		PackageScripts:     []domain.PackageScript{{Name: "dev"}},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{SkipServices: true}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SkipServices {
		t.Error("SkipServices = false, want true")
	}
	if len(got.DockerComposeFiles) != 0 || len(got.SelectedPackageScripts) != 0 {
		t.Errorf("services not skipped: docker=%v scripts=%v", got.DockerComposeFiles, got.SelectedPackageScripts)
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
	// on_create = bare install + one hook per monorepo package.
	if len(got.OnCreate) != 3 {
		t.Fatalf("expected 3 on_create hooks, got %d", len(got.OnCreate))
	}
	if got.OnCreate[0].Cmd != "pnpm install" || got.OnCreate[0].Cwd != "" {
		t.Errorf("first hook should be the bare install command: %+v", got.OnCreate[0])
	}
	if got.OnCreate[1].Cwd != "packages/a" || got.OnCreate[2].Cwd != "packages/b" {
		t.Errorf("monorepo hooks missing: %+v", got.OnCreate)
	}
}
