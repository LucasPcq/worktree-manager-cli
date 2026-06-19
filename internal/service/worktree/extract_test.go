package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// extractEnv holds the two worktrees used by the extract tests.
type extractEnv struct {
	source string
	target string
}

// setupExtract builds a repo with files a.txt and b.txt committed, plus a target
// worktree branched from the same HEAD.
func setupExtract(t *testing.T) extractEnv {
	t.Helper()
	source := gittest.InitRepo(t)
	writeFile(t, source, "a.txt", "line1\nline2\nline3\n")
	writeFile(t, source, "b.txt", "base\n")
	gitRun(t, source, "add", ".")
	gitRun(t, source, "commit", "-qm", "files")

	target := filepath.Join(t.TempDir(), "tgt")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", target, "HEAD")

	return extractEnv{source: source, target: target}
}

func TestExtractMovesModifiedFile(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nCHANGED\nline3\n")

	result, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "a.txt"); got != "line1\nCHANGED\nline3\n" {
		t.Errorf("target a.txt = %q, want changed content", got)
	}
	if got := readFile(t, env.source, "a.txt"); got != "line1\nline2\nline3\n" {
		t.Errorf("source a.txt = %q, want restored to HEAD", got)
	}
	if status := gitStatus(t, env.source); status != "" {
		t.Errorf("source not clean after move: %q", status)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "a.txt" {
		t.Errorf("result.Files = %v", result.Files)
	}
}

func TestExtractMovesUntrackedFile(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "brand new\n")

	_, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "c.txt"); got != "brand new\n" {
		t.Errorf("target c.txt = %q", got)
	}
	if fileExists(env.source, "c.txt") {
		t.Error("source c.txt should be removed after move")
	}
}

func TestExtractMovesDeletedFile(t *testing.T) {
	env := setupExtract(t)
	if err := os.Remove(filepath.Join(env.source, "b.txt")); err != nil {
		t.Fatal(err)
	}

	_, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "b.txt", Status: domain.ExtractStatusDeleted}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if fileExists(env.target, "b.txt") {
		t.Error("target b.txt should be deleted")
	}
	if !fileExists(env.source, "b.txt") {
		t.Error("source b.txt should be restored to HEAD")
	}
}

func TestExtractKeepLeavesSource(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nKEEP\nline3\n")

	_, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Keep:       true,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.source, "a.txt"); got != "line1\nKEEP\nline3\n" {
		t.Errorf("source a.txt = %q, want kept", got)
	}
	if got := readFile(t, env.target, "a.txt"); got != "line1\nKEEP\nline3\n" {
		t.Errorf("target a.txt = %q, want copied", got)
	}
}

func TestExtractConflictAbortsAndLeavesSourceIntact(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nSRC\nline3\n")
	writeFile(t, env.target, "a.txt", "line1\nTGT\nline3\n")

	_, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("err = %v, want ErrExtractConflict", err)
	}

	if got := readFile(t, env.source, "a.txt"); got != "line1\nSRC\nline3\n" {
		t.Errorf("source a.txt changed on conflict: %q", got)
	}
	if got := readFile(t, env.target, "a.txt"); got != "line1\nTGT\nline3\n" {
		t.Errorf("target a.txt changed on conflict: %q", got)
	}
}

func TestExtractUntrackedCollisionAborts(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "from source\n")
	writeFile(t, env.target, "c.txt", "already here\n")

	_, err := Extract(domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("err = %v, want ErrExtractConflict", err)
	}
	if got := readFile(t, env.target, "c.txt"); got != "already here\n" {
		t.Errorf("target c.txt clobbered: %q", got)
	}
}

func TestExtractGuards(t *testing.T) {
	env := setupExtract(t)

	_, err := Extract(domain.ExtractParams{SourcePath: env.source, TargetPath: env.source})
	if !errors.Is(err, domain.ErrSameWorktree) {
		t.Errorf("same worktree: err = %v", err)
	}

	_, err = Extract(domain.ExtractParams{SourcePath: env.source, TargetPath: env.target})
	if !errors.Is(err, domain.ErrNoFilesSelected) {
		t.Errorf("no files: err = %v", err)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func fileExists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, rel))
	return err == nil
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(out)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
