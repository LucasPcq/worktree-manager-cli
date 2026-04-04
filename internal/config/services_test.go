package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const servicesTestToml = `
[[services]]
name = "api"
cmd = "docker-compose up api"
stop = "docker-compose stop api"
cwd = "."

[[services]]
name = "web"
cmd = "pnpm dev"
cwd = "apps/web"

[[profiles]]
name = "full"
services = ["api", "web"]
default = true

[[profiles]]
name = "back"
services = ["api"]
`

func writeServicesFile(t *testing.T, dir string, content string) {
	t.Helper()
	wtmDir := filepath.Join(dir, domain.ProjectDirName)
	os.MkdirAll(wtmDir, 0o755)
	if err := os.WriteFile(filepath.Join(wtmDir, domain.ServicesFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadServicesFullConfig(t *testing.T) {
	dir := t.TempDir()
	writeServicesFile(t, dir, servicesTestToml)

	cfg, err := LoadServices(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "api" {
		t.Errorf("expected first service name=api, got %s", cfg.Services[0].Name)
	}
	if cfg.Services[1].Cwd != "apps/web" {
		t.Errorf("expected web cwd=apps/web, got %s", cfg.Services[1].Cwd)
	}

	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}

	defaultProfile, ok := cfg.DefaultProfile()
	if !ok {
		t.Fatal("expected default profile")
	}
	if defaultProfile.Name != "full" {
		t.Errorf("expected default profile=full, got %s", defaultProfile.Name)
	}
}

func TestLoadServicesMissingFile(t *testing.T) {
	cfg, err := LoadServices(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 0 {
		t.Error("expected empty services for missing file")
	}
}

func TestFindService(t *testing.T) {
	dir := t.TempDir()
	writeServicesFile(t, dir, servicesTestToml)

	cfg, _ := LoadServices(dir)

	svc, ok := cfg.FindService("api")
	if !ok {
		t.Fatal("expected to find api service")
	}
	if svc.Cmd != "docker-compose up api" {
		t.Errorf("unexpected cmd: %s", svc.Cmd)
	}

	_, ok = cfg.FindService("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent service")
	}
}

func TestProfileServices(t *testing.T) {
	dir := t.TempDir()
	writeServicesFile(t, dir, servicesTestToml)

	cfg, _ := LoadServices(dir)

	back, ok := cfg.FindProfile("back")
	if !ok {
		t.Fatal("expected to find back profile")
	}

	services := cfg.ProfileServices(back)
	if len(services) != 1 {
		t.Fatalf("expected 1 service in back profile, got %d", len(services))
	}
	if services[0].Name != "api" {
		t.Errorf("expected api, got %s", services[0].Name)
	}
}

func TestDefaultProfileReturnsDefault(t *testing.T) {
	cfg := domain.ServicesConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "first", Default: false},
			{Name: "second", Default: true},
		},
	}

	p, ok := cfg.DefaultProfile()
	if !ok {
		t.Fatal("expected to find a default profile")
	}
	if p.Name != "second" {
		t.Errorf("expected default profile=second, got %s", p.Name)
	}
}

func TestDefaultProfileFallbackFirst(t *testing.T) {
	cfg := domain.ServicesConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}

	p, ok := cfg.DefaultProfile()
	if !ok {
		t.Fatal("expected fallback to first profile")
	}
	if p.Name != "alpha" {
		t.Errorf("expected fallback profile=alpha, got %s", p.Name)
	}
}

func TestDefaultProfileEmpty(t *testing.T) {
	cfg := domain.ServicesConfig{}

	_, ok := cfg.DefaultProfile()
	if ok {
		t.Error("expected no default profile for empty config")
	}
}

func TestValidateServicesDuplicateServiceNames(t *testing.T) {
	cfg := domain.ServicesConfig{
		Services: []domain.ServiceConfig{
			{Name: "api", Cmd: "echo 1"},
			{Name: "api", Cmd: "echo 2"},
		},
	}
	warnings := ValidateServices(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateServicesDuplicateProfileNames(t *testing.T) {
	cfg := domain.ServicesConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "full"},
			{Name: "full"},
		},
	}
	warnings := ValidateServices(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateServicesMultipleDefaults(t *testing.T) {
	cfg := domain.ServicesConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "a", Default: true},
			{Name: "b", Default: true},
		},
	}
	warnings := ValidateServices(cfg)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateServicesCleanConfig(t *testing.T) {
	cfg := domain.ServicesConfig{
		Services: []domain.ServiceConfig{
			{Name: "api", Cmd: "echo"},
			{Name: "web", Cmd: "echo"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Default: true},
			{Name: "back"},
		},
	}
	warnings := ValidateServices(cfg)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
}
