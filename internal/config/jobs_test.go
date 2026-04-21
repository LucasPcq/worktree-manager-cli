package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestLoadRunRejectsTypoedSection(t *testing.T) {
	dir := t.TempDir()
	wtmDir := filepath.Join(dir, domain.ProjectDirName)
	if err := os.MkdirAll(wtmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wtmDir, domain.RunFileName)
	body := `
[[job]]
name = "dev"
kind = "service"
cmd = "pnpm dev"

[[profiles]]
name = "full"
jobs = ["dev"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected error for typoed [[profiles]] section")
	}
	if !strings.Contains(err.Error(), "profiles") {
		t.Errorf("expected error to mention 'profiles', got: %v", err)
	}
}


func TestBuildScriptJobsRoot(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Cmd: "vite", Workspace: "", PkgName: "app"},
		{Name: "build", Cmd: "vite build", Workspace: "", PkgName: "app"},
		{Name: "test", Cmd: "vitest", Workspace: "", PkgName: "app"},
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        scripts,
	})

	if len(cfg.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(cfg.Jobs))
	}

	devJob := cfg.Jobs[0]
	if devJob.Name != "dev" {
		t.Errorf("expected name=dev, got %q", devJob.Name)
	}
	if devJob.Kind != domain.JobKindService {
		t.Errorf("expected kind=service, got %q", devJob.Kind)
	}
	if devJob.Cmd != "pnpm run dev" {
		t.Errorf("expected cmd=pnpm run dev, got %q", devJob.Cmd)
	}
	if devJob.Cwd != "." {
		t.Errorf("expected cwd=., got %q", devJob.Cwd)
	}
	if devJob.Stop != "" {
		t.Error("expected no stop command on PID-tracked service")
	}

	buildJob := cfg.Jobs[1]
	if buildJob.Kind != domain.JobKindTask {
		t.Errorf("expected build kind=task, got %q", buildJob.Kind)
	}
}

func TestBuildScriptJobsWorkspace(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Cmd: "next dev", Workspace: "apps/web", PkgName: "web"},
		{Name: "dev", Cmd: "tsx watch src", Workspace: "apps/api", PkgName: "api"},
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerNpm,
		Scripts:        scripts,
	})

	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(cfg.Jobs))
	}

	if cfg.Jobs[0].Name != "web-dev" {
		t.Errorf("expected web-dev, got %q", cfg.Jobs[0].Name)
	}
	if cfg.Jobs[0].Cwd != "apps/web" {
		t.Errorf("expected cwd=apps/web, got %q", cfg.Jobs[0].Cwd)
	}
	if cfg.Jobs[0].Cmd != "npm run dev" {
		t.Errorf("expected npm run dev, got %q", cfg.Jobs[0].Cmd)
	}
	if cfg.Jobs[1].Name != "api-dev" {
		t.Errorf("expected api-dev, got %q", cfg.Jobs[1].Name)
	}
}

func TestBuildScriptJobsNameDedup(t *testing.T) {
	// Two root scripts that produce the same base name after scoping (shouldn't
	// happen in practice, but the counter prevents a broken run.toml).
	scripts := []domain.PackageScript{
		{Name: "dev", Workspace: "", PkgName: "a"},
		{Name: "dev", Workspace: "", PkgName: "b"}, // same base "dev"
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerYarn,
		Scripts:        scripts,
	})

	if cfg.Jobs[0].Name == cfg.Jobs[1].Name {
		t.Errorf("expected deduped names, both are %q", cfg.Jobs[0].Name)
	}
}

func TestBuildDockerJobsKind(t *testing.T) {
	cfg := BuildDockerJobs("docker compose", []string{"docker-compose.yml"})
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	j := cfg.Jobs[0]
	if j.Kind != domain.JobKindService {
		t.Errorf("expected Kind=service, got %q", j.Kind)
	}
	if j.Stop == "" {
		t.Error("expected stop command on docker job")
	}
	if !rules.IsDetached(j) {
		t.Error("expected docker job to be detached")
	}
}
