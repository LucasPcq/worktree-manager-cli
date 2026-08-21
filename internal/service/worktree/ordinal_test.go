package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type ordinalRepo struct {
	dir      string
	stateDir string
}

func newOrdinalRepo(t *testing.T) ordinalRepo {
	t.Helper()
	dir := gittest.InitRepo(t)
	return ordinalRepo{dir: dir, stateDir: filepath.Join(dir, ".git", "wtm")}
}

func (r ordinalRepo) addWorktree(t *testing.T, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), rules.SanitizeBranchName(branch))
	git(t, r.dir, "worktree", "add", "-b", branch, path)
	return path
}

func (r ordinalRepo) ensure(t *testing.T, branch string) int {
	t.Helper()
	ordinal, err := EnsureOrdinal(EnsureOrdinalParams{
		ProjectDir: r.dir,
		StateDir:   r.stateDir,
		Branch:     branch,
	})
	if err != nil {
		t.Fatalf("EnsureOrdinal(%s): %v", branch, err)
	}
	return ordinal
}

func (r ordinalRepo) meta(t *testing.T, branch string) domain.WorktreeMetadata {
	t.Helper()
	meta, err := loadMetadata(r.stateDir, branch)
	if err != nil {
		t.Fatalf("loadMetadata(%s): %v", branch, err)
	}
	return meta
}

func TestEnsureOrdinalMainKeepsZeroAndWritesNothing(t *testing.T) {
	repo := newOrdinalRepo(t)

	if got := repo.ensure(t, "main"); got != domain.MainWorktreeOrdinal {
		t.Errorf("main ordinal = %d, want %d", got, domain.MainWorktreeOrdinal)
	}

	metaPath := filepath.Join(rules.WorktreeMetaDir(repo.stateDir, "main"), domain.MetaFileName)
	if _, err := os.Stat(metaPath); err == nil {
		t.Error("main worktree got a meta.json, want none")
	}
}

func TestEnsureOrdinalAllocatesAndPersists(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")

	if got := repo.ensure(t, "feat/one"); got != 1 {
		t.Fatalf("first linked worktree = %d, want 1", got)
	}
	if got := repo.meta(t, "feat/one").Ordinal; got != 1 {
		t.Errorf("persisted ordinal = %d, want 1", got)
	}

	// Idempotent: asking again returns the number already recorded.
	if got := repo.ensure(t, "feat/one"); got != 1 {
		t.Errorf("second call = %d, want 1", got)
	}
}

func TestEnsureOrdinalGivesEachWorktreeItsOwn(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")
	repo.addWorktree(t, "feat/two")

	first := repo.ensure(t, "feat/one")
	second := repo.ensure(t, "feat/two")

	if first == second {
		t.Fatalf("both worktrees got ordinal %d", first)
	}
	if first != 1 || second != 2 {
		t.Errorf("ordinals = %d/%d, want 1/2", first, second)
	}
}

func TestEnsureOrdinalRecyclesRemovedWorktreeNumber(t *testing.T) {
	repo := newOrdinalRepo(t)
	gone := repo.addWorktree(t, "feat/gone")
	repo.addWorktree(t, "feat/two")

	repo.ensure(t, "feat/gone")
	if got := repo.ensure(t, "feat/two"); got != 2 {
		t.Fatalf("feat/two = %d, want 2", got)
	}

	// The worktree goes away but its meta.json stays behind, as it does for any
	// removal wtm did not drive. A stale meta must not reserve a number.
	git(t, repo.dir, "worktree", "remove", "--force", gone)
	repo.addWorktree(t, "feat/three")

	if got := repo.ensure(t, "feat/three"); got != 1 {
		t.Errorf("feat/three = %d, want 1 (the freed number)", got)
	}
}

func TestEnsureOrdinalBackfillsWithoutLosingMetadata(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/external")
	writeMeta(t, repo.stateDir, "feat/external", "main")

	before := repo.meta(t, "feat/external")
	if before.Ordinal != 0 {
		t.Fatalf("fixture already has ordinal %d", before.Ordinal)
	}

	if got := repo.ensure(t, "feat/external"); got != 1 {
		t.Fatalf("backfilled ordinal = %d, want 1", got)
	}

	after := repo.meta(t, "feat/external")
	if after.SourceBranch != before.SourceBranch {
		t.Errorf("source_branch = %q, want %q", after.SourceBranch, before.SourceBranch)
	}
	if after.CreatedAt != before.CreatedAt {
		t.Errorf("created_at = %q, want %q", after.CreatedAt, before.CreatedAt)
	}
}

func TestEnsureOrdinalRepairsDuplicate(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")
	repo.addWorktree(t, "feat/clone")

	repo.ensure(t, "feat/one")

	// A meta.json copied by hand, or a lock that could not be taken: two live
	// worktrees claiming the same number.
	duplicate := domain.WorktreeMetadata{SourceBranch: "main", Ordinal: 1}
	if err := writeMetadata(rules.WorktreeMetaDir(repo.stateDir, "feat/clone"), duplicate); err != nil {
		t.Fatalf("write duplicate: %v", err)
	}

	if got := repo.ensure(t, "feat/clone"); got == 1 {
		t.Fatal("duplicate ordinal kept, want re-allocation")
	}
	if got := repo.meta(t, "feat/clone").Ordinal; got != 2 {
		t.Errorf("repaired ordinal = %d, want 2", got)
	}
	if got := repo.ensure(t, "feat/one"); got != 1 {
		t.Errorf("feat/one = %d, want 1 (it was there first and keeps its number)", got)
	}
}
