// Package gittest provides helpers for creating real git repositories in tests.
package gittest

import (
	"os/exec"
	"strings"
	"testing"
)

// InitRepo creates a temporary git repository with an initial empty commit.
// The repository path is returned; cleanup is handled by t.TempDir().
func InitRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %s: %v", strings.Join(args, " "), out, err)
		}
	}

	return dir
}

func CreateBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "branch", name)
}

// Git runs a git command in dir, failing the test on error.
func Git(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}
