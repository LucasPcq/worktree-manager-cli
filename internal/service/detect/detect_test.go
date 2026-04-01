package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func touchFile(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackageManagerPnpm(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePnpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPnpm {
		t.Errorf("expected pnpm, got %s", pm)
	}
}

func TestPackageManagerNpm(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileNpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerNpm {
		t.Errorf("expected npm, got %s", pm)
	}
}

func TestPackageManagerYarn(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileYarn))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerYarn {
		t.Errorf("expected yarn, got %s", pm)
	}
}

func TestPackageManagerGo(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileGo))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerGo {
		t.Errorf("expected go, got %s", pm)
	}
}

func TestPackageManagerPip(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePip))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPip {
		t.Errorf("expected pip, got %s", pm)
	}
}

func TestPackageManagerNone(t *testing.T) {
	dir := t.TempDir()

	pm := PackageManager(dir)
	if pm != domain.PkgManagerNone {
		t.Errorf("expected none, got %s", pm)
	}
}

func TestPackageManagerPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePnpm))
	touchFile(t, filepath.Join(dir, domain.LockfileNpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPnpm {
		t.Errorf("expected pnpm (higher priority), got %s", pm)
	}
}

func TestInstallCommand(t *testing.T) {
	tests := []struct {
		pm   domain.PackageManager
		want string
	}{
		{domain.PkgManagerPnpm, domain.InstallCmdPnpm},
		{domain.PkgManagerNpm, domain.InstallCmdNpm},
		{domain.PkgManagerYarn, domain.InstallCmdYarn},
		{domain.PkgManagerGo, domain.InstallCmdGo},
		{domain.PkgManagerPip, domain.InstallCmdPip},
		{domain.PkgManagerNone, ""},
	}

	for _, tt := range tests {
		got := InstallCommand(tt.pm)
		if got != tt.want {
			t.Errorf("InstallCommand(%s) = %q, want %q", tt.pm, got, tt.want)
		}
	}
}

func TestEnvFilesFindsRootAndNested(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env"))
	touchFile(t, filepath.Join(dir, ".env.example"))
	touchFile(t, filepath.Join(dir, "apps", "api", ".env"))
	touchFile(t, filepath.Join(dir, "apps", "api", ".env.example"))

	files := EnvFiles(dir)
	if len(files) != 4 {
		t.Fatalf("expected 4 env files, got %d: %v", len(files), files)
	}
}

func TestEnvFilesExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env"))
	touchFile(t, filepath.Join(dir, "node_modules", ".env"))

	files := EnvFiles(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 env file (excluded node_modules), got %d: %v", len(files), files)
	}
}

func TestEnvFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	files := EnvFiles(dir)
	if len(files) != 0 {
		t.Fatalf("expected 0 env files, got %d: %v", len(files), files)
	}
}

func TestProjectConfigExists(t *testing.T) {
	dir := t.TempDir()
	if ProjectConfigExists(dir) {
		t.Error("expected false for empty dir")
	}

	touchFile(t, filepath.Join(dir, domain.ConfigFileName))
	if !ProjectConfigExists(dir) {
		t.Error("expected true after creating .wtm.toml")
	}
}

func TestBaseBranchFallback(t *testing.T) {
	dir := t.TempDir()

	branch := BaseBranch(dir)
	if branch != domain.DefaultBaseBranch {
		t.Errorf("expected fallback %s, got %s", domain.DefaultBaseBranch, branch)
	}
}
