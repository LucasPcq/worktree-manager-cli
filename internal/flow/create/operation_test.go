package create

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

func TestOperationDeclaresHowTheRunHoldsItsSurface(t *testing.T) {
	op := Operation()
	if op.Mode != flow.ModeBackground {
		t.Errorf("create mode = %v, want background: the on_create hooks can run long", op.Mode)
	}
	if op.TargetKey != KeyBranch {
		t.Errorf("target key = %q, want the answer naming the branch being provisioned", op.TargetKey)
	}
	if op.Kind != domain.OpKindCreate {
		t.Errorf("kind = %q, want %q", op.Kind, domain.OpKindCreate)
	}
}
