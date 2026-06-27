package reparent

import (
	"strings"
	"testing"
)

func TestParentStepDescriptionShowsCurrentParent(t *testing.T) {
	desc := parentStepDescription("dev/a")
	if !strings.Contains(desc, "dev/a") {
		t.Fatalf("description should name the current parent, got %q", desc)
	}
}

func TestParentStepDescriptionHandlesNoParent(t *testing.T) {
	// A merged-then-cleaned parent leaves no recorded value; the description must
	// still render (no badge that would silently vanish).
	desc := parentStepDescription("")
	if desc == "" || strings.Contains(desc, "Currently rebased onto") {
		t.Fatalf("empty parent should yield a distinct no-parent description, got %q", desc)
	}
}

func TestParentItemsExcludesChosenWorktree(t *testing.T) {
	branches := []string{"main", "feat", "dev-a", "dev-b"}

	items := parentItems(branches, "dev-b")

	if len(items) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %+v", len(items), items)
	}
	for _, item := range items {
		if item.Value == "dev-b" {
			t.Fatalf("chosen worktree dev-b must not be a parent candidate")
		}
	}
}

func TestParentItemsKeepsAllWhenNoExclusion(t *testing.T) {
	branches := []string{"main", "feat"}

	if items := parentItems(branches, ""); len(items) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(items))
	}
}
