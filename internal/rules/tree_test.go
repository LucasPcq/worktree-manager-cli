package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func at(day int) time.Time {
	return time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)
}

func findRoot(forest domain.Forest, branch string) (domain.TreeNode, bool) {
	for _, r := range forest.Roots {
		if r.Branch == branch {
			return r, true
		}
	}
	return domain.TreeNode{}, false
}

func childBranches(node domain.TreeNode) []string {
	out := make([]string, 0, len(node.Children))
	for _, c := range node.Children {
		out = append(out, c.Branch)
	}
	return out
}

func TestBuildForestSimpleStack(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "feat", SourceBranch: "main", CreatedAt: at(2)},
			{Branch: "feat-ui", SourceBranch: "feat", CreatedAt: at(3)},
		},
	})

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d: %+v", len(forest.Roots), forest.Roots)
	}
	main := forest.Roots[0]
	if main.Branch != "main" || !main.IsMain {
		t.Fatalf("expected main root, got %+v", main)
	}
	if got := childBranches(main); len(got) != 1 || got[0] != "feat" {
		t.Fatalf("expected [feat] under main, got %v", got)
	}
	if got := childBranches(main.Children[0]); len(got) != 1 || got[0] != "feat-ui" {
		t.Fatalf("expected [feat-ui] under feat, got %v", got)
	}
}

func TestBuildForestVirtualRoot(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "spike", SourceBranch: "dev", CreatedAt: at(2)},
		},
	})

	dev, ok := findRoot(forest, "dev")
	if !ok {
		t.Fatalf("expected virtual root 'dev', got roots %+v", forest.Roots)
	}
	if !dev.IsVirtual || dev.Path != "" {
		t.Fatalf("expected virtual dev root with no path, got %+v", dev)
	}
	if got := childBranches(dev); len(got) != 1 || got[0] != "spike" {
		t.Fatalf("expected [spike] under dev, got %v", got)
	}
}

func TestBuildForestMainFirstThenVirtual(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "spike", SourceBranch: "dev", CreatedAt: at(2)},
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
		},
	})

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(forest.Roots))
	}
	if !forest.Roots[0].IsMain {
		t.Fatalf("expected main first, got %+v", forest.Roots[0])
	}
	if !forest.Roots[1].IsVirtual {
		t.Fatalf("expected virtual root last, got %+v", forest.Roots[1])
	}
}

func TestBuildForestSiblingsByCreatedAt(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "younger", SourceBranch: "main", CreatedAt: at(5)},
			{Branch: "older", SourceBranch: "main", CreatedAt: at(2)},
		},
	})

	got := childBranches(forest.Roots[0])
	if len(got) != 2 || got[0] != "older" || got[1] != "younger" {
		t.Fatalf("expected [older younger] by creation date, got %v", got)
	}
}

func TestBuildForestCycleDoesNotPanic(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "a", SourceBranch: "b", CreatedAt: at(2)},
			{Branch: "b", SourceBranch: "a", CreatedAt: at(3)},
		},
	})

	cycleFlagged := false
	var walk func(n domain.TreeNode)
	walk = func(n domain.TreeNode) {
		if n.Status.InCycle {
			cycleFlagged = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range forest.Roots {
		walk(r)
	}

	if !cycleFlagged {
		t.Fatalf("expected at least one node flagged InCycle, got %+v", forest.Roots)
	}
}

func TestBuildForestStatusPreserved(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "feat", SourceBranch: "main", CreatedAt: at(2), Status: domain.TreeNodeStatus{
				CommitsAhead: 3,
				IsDirty:      true,
				NeedsSync:    true,
			}},
		},
	})

	feat := forest.Roots[0].Children[0]
	if feat.Status.CommitsAhead != 3 || !feat.Status.IsDirty || !feat.Status.NeedsSync {
		t.Fatalf("status not preserved: %+v", feat.Status)
	}
}
