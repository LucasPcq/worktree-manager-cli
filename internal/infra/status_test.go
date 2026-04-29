package infra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestIsDirty_CleanRepo(t *testing.T) {
	dir := gittest.InitRepo(t)

	dirty, err := IsDirty(IsDirtyParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Error("expected clean repo to not be dirty")
	}
}

func TestIsDirty_DirtyRepo(t *testing.T) {
	dir := gittest.InitRepo(t)

	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello"), 0o644)

	dirty, err := IsDirty(IsDirtyParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Error("expected repo with untracked file to be dirty")
	}
}
