package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func reparentNodes() []domain.WorktreeNode {
	return []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "dev/a", SourceBranch: "feat"},
		{Branch: "dev/b", SourceBranch: "dev/a"},
	}
}

func TestValidateReparentValid(t *testing.T) {
	err := ValidateReparent(ValidateReparentParams{
		Nodes:      reparentNodes(),
		Branch:     "dev/b",
		NewParent:  "feat",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("expected valid reparent, got %v", err)
	}
}

func TestValidateReparentRejectsSelfParent(t *testing.T) {
	err := ValidateReparent(ValidateReparentParams{
		Nodes:      reparentNodes(),
		Branch:     "dev/b",
		NewParent:  "dev/b",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrReparentSelf) {
		t.Fatalf("expected ErrReparentSelf, got %v", err)
	}
}

func TestValidateReparentRejectsCycle(t *testing.T) {
	// Making feat a child of dev/b closes the loop feat → dev/b → dev/a → feat.
	err := ValidateReparent(ValidateReparentParams{
		Nodes:      reparentNodes(),
		Branch:     "feat",
		NewParent:  "dev/b",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatalf("expected a cycle error, got nil")
	}
}

func TestValidateReparentRejectsUnknownBranch(t *testing.T) {
	err := ValidateReparent(ValidateReparentParams{
		Nodes:      reparentNodes(),
		Branch:     "ghost",
		NewParent:  "feat",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Fatalf("expected ErrWorktreeNotFound, got %v", err)
	}
}

func TestValidateReparentBatchValid(t *testing.T) {
	// Flatten the stack: both dev worktrees move onto main in one pass.
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "dev/b"},
		NewParent:  "main",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("expected valid batch reparent, got %v", err)
	}
}

func TestValidateReparentBatchRejectsCycle(t *testing.T) {
	// Moving both feat and dev/a onto dev/b, while dev/b still points at dev/a,
	// closes the loop dev/a → dev/b → dev/a. The batch must validate the combined
	// graph in one pass and reject it.
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"feat", "dev/a"},
		NewParent:  "dev/b",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatalf("expected a cycle error across the batch, got nil")
	}
}

func TestValidateReparentBatchRejectsSelfParent(t *testing.T) {
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "feat"},
		NewParent:  "feat",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrReparentSelf) {
		t.Fatalf("expected ErrReparentSelf when a listed branch equals the new parent, got %v", err)
	}
}

func TestValidateReparentBatchRejectsUnknownBranch(t *testing.T) {
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "ghost"},
		NewParent:  "main",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Fatalf("expected ErrWorktreeNotFound for an unmanaged branch, got %v", err)
	}
}

func TestChildrenOf(t *testing.T) {
	children := ChildrenOf(ChildrenOfParams{Nodes: reparentNodes(), Branch: "feat"})
	if len(children) != 1 || children[0].Branch != "dev/a" {
		t.Fatalf("expected [dev/a], got %+v", children)
	}

	if got := ChildrenOf(ChildrenOfParams{Nodes: reparentNodes(), Branch: "dev/b"}); len(got) != 0 {
		t.Fatalf("expected no children for leaf dev/b, got %+v", got)
	}
}
