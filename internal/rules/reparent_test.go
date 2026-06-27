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

func TestChildrenOf(t *testing.T) {
	children := ChildrenOf(reparentNodes(), "feat")
	if len(children) != 1 || children[0].Branch != "dev/a" {
		t.Fatalf("expected [dev/a], got %+v", children)
	}

	if got := ChildrenOf(reparentNodes(), "dev/b"); len(got) != 0 {
		t.Fatalf("expected no children for leaf dev/b, got %+v", got)
	}
}
