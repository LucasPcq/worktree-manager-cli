package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSyncSubtreeReturnsRootAndDescendants(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat-a", SourceBranch: "main"},
		{Branch: "feat-b", SourceBranch: "feat-a"},
		{Branch: "feat-c", SourceBranch: "feat-b"},
		{Branch: "solo", SourceBranch: "main"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "feat-a"})

	want := "feat-a,feat-b,feat-c"
	if strings.Join(got, ",") != want {
		t.Fatalf("SyncSubtree(feat-a) = %v, want %s", got, want)
	}
}

func TestSyncSubtreeLeafReturnsItself(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat-a", SourceBranch: "main"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "feat-a"})

	if len(got) != 1 || got[0] != "feat-a" {
		t.Fatalf("SyncSubtree on a leaf = %v, want [feat-a]", got)
	}
}

func TestSyncSubtreeUnknownRootReturnsNothing(t *testing.T) {
	nodes := []domain.WorktreeNode{{Branch: "feat-a", SourceBranch: "main"}}

	if got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "ghost"}); len(got) != 0 {
		t.Fatalf("SyncSubtree on an unknown root = %v, want nothing", got)
	}
}

// Un cycle dans la chaîne des parents ne doit pas boucler : la règle est appelée
// depuis un rendu de menu, elle ne peut pas se permettre de ne pas rendre la main.
func TestSyncSubtreeTerminatesOnACycle(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "a", SourceBranch: "b"},
		{Branch: "b", SourceBranch: "a"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "a"})

	if len(got) != 2 {
		t.Fatalf("SyncSubtree on a cycle = %v, want both branches once", got)
	}
}
