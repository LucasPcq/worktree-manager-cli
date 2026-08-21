package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestLoadRunRejectsTypoedSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, domain.RunFileName)
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

func writeRunFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, domain.RunFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadRunReadsPorts(t *testing.T) {
	dir := writeRunFile(t, `
port_offset_block = 100

[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
  [job.ports]
  PORT = 3000

[[job]]
name = "db"
kind = "service"
cmd = "docker compose up -d"
ports = { DB_PORT = 5432 }
`)

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PortOffsetBlock != 100 {
		t.Errorf("got block %d, want 100", cfg.PortOffsetBlock)
	}
	if cfg.Jobs[0].Ports["PORT"] != 3000 {
		t.Errorf("sub-table form: got %v", cfg.Jobs[0].Ports)
	}
	// The encoder only ever writes the sub-table form, but a hand-written inline
	// table is the same TOML and must keep loading.
	if cfg.Jobs[1].Ports["DB_PORT"] != 5432 {
		t.Errorf("inline form: got %v", cfg.Jobs[1].Ports)
	}
}

func TestLoadRunRejectsCollidingPorts(t *testing.T) {
	dir := writeRunFile(t, `
[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 3000, ADMIN = 3010 }
`)

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected the colliding bases to be refused at load")
	}
	for _, want := range []string{"PORT", "ADMIN", "3000", "3010"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// The structural checks stay on write: a file with a duplicate job name must
// still load, or `run job rm` could not repair it.
func TestLoadRunAcceptsDuplicateJobName(t *testing.T) {
	dir := writeRunFile(t, `
[[job]]
name = "dev"
kind = "service"
cmd = "pnpm dev"

[[job]]
name = "dev"
kind = "service"
cmd = "echo dup"
`)

	if _, err := LoadRun(dir); err != nil {
		t.Fatalf("structural errors must not block a read: %v", err)
	}
}
