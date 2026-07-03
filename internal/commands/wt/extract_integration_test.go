package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// extractTestRepo sets up a git repo + minimal wtm config and returns the main
// worktree dir and state dir.
func extractTestRepo(t *testing.T) (dir, stateDir string) {
	t.Helper()
	dir = gittest.InitRepo(t)
	stateDir = filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return dir, stateDir
}

func createWorktree(t *testing.T, branch string) domain.CreateResult {
	t.Helper()
	out, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	var res domain.CreateResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode create result: %v\n%s", err, out)
	}
	return res
}

// TestExtractRequiresSourceNonInteractive verifies that, aligned with sync, a
// non-interactive extract without a source argument errors instead of defaulting
// to the current worktree.
func TestExtractRequiresSourceNonInteractive(t *testing.T) {
	_, _ = extractTestRepo(t)

	_, _, err := runWtCmd(t, domain.CmdExtract, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrExtractSourceRequired) {
		t.Fatalf("expected ErrExtractSourceRequired, got %v", err)
	}
}

// TestExtractUnknownSource verifies a source argument that is not a worktree
// branch is rejected.
func TestExtractUnknownSource(t *testing.T) {
	_, _ = extractTestRepo(t)

	_, _, err := runWtCmd(t, domain.CmdExtract, "does-not-exist", "--files", "x", "--to", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected an error for an unknown source worktree")
	}
}

// TestExtractYesRequiresSelectionFlags verifies the hardened bypass taxonomy: under
// --yes a missing required selection errors naming the flag (no picker fallback),
// while --yes with every selection supplied runs unattended.
func TestExtractYesRequiresSelectionFlags(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	createWorktree(t, "dst")

	if err := os.WriteFile(filepath.Join(src.Path, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	// --yes without --files errors (files are a required selection with no default).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--to", "dst", "--yes"); !errors.Is(err, domain.ErrExtractFilesRequired) {
		t.Fatalf("expected ErrExtractFilesRequired, got %v", err)
	}
	// --yes without --to errors (target is a required selection with no default).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "f.txt", "--yes"); !errors.Is(err, domain.ErrExtractTargetRequired) {
		t.Fatalf("expected ErrExtractTargetRequired, got %v", err)
	}
	// --yes with source + --files + --to runs unattended (text output, no picker).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "f.txt", "--to", "dst", "--yes"); err != nil {
		t.Fatalf("extract --yes with full flags: %v", err)
	}
}

// TestExtractWithSourceArgJSON drives the full non-interactive extraction: a
// source worktree is named by argument, its uncommitted file is moved to the
// target worktree, and the JSON result reports the source branch.
func TestExtractWithSourceArgJSON(t *testing.T) {
	_, _ = extractTestRepo(t)

	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	srcFile := filepath.Join(src.Path, "extracted.txt")
	if err := os.WriteFile(srcFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	out, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "extracted.txt", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if res.SourceBranch != "src" {
		t.Errorf("source_branch = %q, want \"src\"", res.SourceBranch)
	}
	if res.TargetBranch != "dst" {
		t.Errorf("target_branch = %q, want \"dst\"", res.TargetBranch)
	}

	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("expected the file to be moved out of the source, stat: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst.Path, "extracted.txt")); err != nil {
		t.Errorf("expected the file in the target worktree: %v", err)
	}
}
