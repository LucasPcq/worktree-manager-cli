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

	// autocrlf/eol are pinned so fixtures written with \n stay byte-identical on
	// disk. Windows runners default to core.autocrlf=true, which rewrites the
	// working tree to CRLF and makes `git status` report phantom modifications
	// on files whose content git considers unchanged (`git diff` stays empty).
	commands := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "core.autocrlf", "false"},
		{"git", "config", "core.eol", "lf"},
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

// CreateBranch creates a new local branch in the given repo.
func CreateBranch(t testing.TB, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "branch", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch %s: %s: %v", name, out, err)
	}
}
