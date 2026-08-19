package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

var ancestryNodes = []domain.WorktreeNode{
	{Branch: "main", IsMain: true},
	{Branch: "feat-a", SourceBranch: "main"},
	{Branch: "feat-b", SourceBranch: "feat-a"},
	{Branch: "feat-c", SourceBranch: "feat-b"},
	{Branch: "solo", SourceBranch: "main"},
}

// The base leads: it is fast-forwarded from its remote before anything rebases
// onto it, so a leaf asked for alone still lands on an up-to-date chain.
func TestSyncAncestryLeadsWithTheRootAndEndsOnTheLeaf(t *testing.T) {
	got := SyncAncestry(SyncAncestryParams{Nodes: ancestryNodes, Leaf: "feat-c"})

	want := "main,feat-a,feat-b,feat-c"
	if strings.Join(got, ",") != want {
		t.Fatalf("SyncAncestry(feat-c) = %v, want %s", got, want)
	}
}

// Descendants are somebody else's gesture: syncing a parent must not drag the
// children the user did not point at.
func TestSyncAncestryLeavesDescendantsOut(t *testing.T) {
	got := SyncAncestry(SyncAncestryParams{Nodes: ancestryNodes, Leaf: "feat-a"})

	want := "main,feat-a"
	if strings.Join(got, ",") != want {
		t.Fatalf("SyncAncestry(feat-a) = %v, want %s", got, want)
	}
}

func TestSyncAncestryOfTheRootIsItself(t *testing.T) {
	got := SyncAncestry(SyncAncestryParams{Nodes: ancestryNodes, Leaf: "main"})

	if len(got) != 1 || got[0] != "main" {
		t.Fatalf("SyncAncestry(main) = %v, want [main]", got)
	}
}

// A parent that is not managed stops the walk: nothing outside the forest can be
// refreshed by this run.
func TestSyncAncestryStopsAtAnUnmanagedParent(t *testing.T) {
	nodes := []domain.WorktreeNode{{Branch: "feat-a", SourceBranch: "upstream/main"}}

	got := SyncAncestry(SyncAncestryParams{Nodes: nodes, Leaf: "feat-a"})

	if len(got) != 1 || got[0] != "feat-a" {
		t.Fatalf("SyncAncestry with an unmanaged parent = %v, want [feat-a]", got)
	}
}

func TestSyncAncestryUnknownLeafReturnsNothing(t *testing.T) {
	if got := SyncAncestry(SyncAncestryParams{Nodes: ancestryNodes, Leaf: "ghost"}); len(got) != 0 {
		t.Fatalf("SyncAncestry on an unknown leaf = %v, want nothing", got)
	}
}

// The walk is called from a menu render: a cycle in the recorded parents must not
// keep it from handing back.
func TestSyncAncestryTerminatesOnACycle(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "a", SourceBranch: "b"},
		{Branch: "b", SourceBranch: "a"},
	}

	got := SyncAncestry(SyncAncestryParams{Nodes: nodes, Leaf: "a"})

	if len(got) != 2 {
		t.Fatalf("SyncAncestry on a cycle = %v, want both branches once", got)
	}
}
