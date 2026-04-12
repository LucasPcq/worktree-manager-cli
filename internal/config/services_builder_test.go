package config

import "testing"

func TestBuildDockerServices_Naming(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"docker-compose.yml", "docker-compose"},
		{"docker-compose.yaml", "docker-compose"},
		{"docker-compose.dev.yml", "docker-compose-dev"},
		{"docker-compose.prod.yaml", "docker-compose-prod"},
	}
	for _, tc := range cases {
		cfg := BuildDockerServices("docker compose", []string{tc.file})
		if len(cfg.Services) != 1 {
			t.Fatalf("%s: expected 1 service, got %d", tc.file, len(cfg.Services))
		}
		if cfg.Services[0].Name != tc.want {
			t.Errorf("%s: got name %q, want %q", tc.file, cfg.Services[0].Name, tc.want)
		}
	}
}

func TestBuildDockerServices_CommandAndStop(t *testing.T) {
	cfg := BuildDockerServices("docker compose", []string{"docker-compose.yml"})
	svc := cfg.Services[0]
	if svc.Cmd != "docker compose -f docker-compose.yml up -d" {
		t.Errorf("cmd: got %q", svc.Cmd)
	}
	if svc.Stop != "docker compose -f docker-compose.yml down --remove-orphans" {
		t.Errorf("stop: got %q", svc.Stop)
	}
	if svc.Cwd != "." {
		t.Errorf("cwd: got %q", svc.Cwd)
	}
}

func TestBuildDockerServices_DuplicateNamesDeduped(t *testing.T) {
	cfg := BuildDockerServices("docker-compose", []string{
		"docker-compose.yml",
		"sub/docker-compose.yaml", // base → also "compose"
	})
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "docker-compose" || cfg.Services[1].Name != "docker-compose-2" {
		t.Errorf("names: got %q, %q — want docker-compose, docker-compose-2", cfg.Services[0].Name, cfg.Services[1].Name)
	}
}

func TestBuildDockerServices_V1Command(t *testing.T) {
	cfg := BuildDockerServices("docker-compose", []string{"docker-compose.yml"})
	if cfg.Services[0].Cmd != "docker-compose -f docker-compose.yml up -d" {
		t.Errorf("v1 cmd: got %q", cfg.Services[0].Cmd)
	}
}

func TestBuildDockerServices_NoProfile(t *testing.T) {
	cfg := BuildDockerServices("docker compose", []string{"docker-compose.yml"})
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected no profiles, got %d", len(cfg.Profiles))
	}
}
