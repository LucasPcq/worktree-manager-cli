package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidatePRForCheckout_Valid(t *testing.T) {
	pr := domain.PRInfo{Number: 1, Branch: "feature/auth", IsFork: false}
	if err := ValidatePRForCheckout(pr, []string{"main", "dev"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePRForCheckout_Fork(t *testing.T) {
	pr := domain.PRInfo{Number: 2, Branch: "feature/auth", IsFork: true}
	err := ValidatePRForCheckout(pr, nil)
	if err == nil {
		t.Error("expected error for fork PR, got nil")
	}
}

func TestValidatePRForCheckout_BranchExists(t *testing.T) {
	pr := domain.PRInfo{Number: 3, Branch: "feature/add-auth", IsFork: false}
	err := ValidatePRForCheckout(pr, []string{"main", "feature/add-auth"})
	if err == nil {
		t.Error("expected error for existing branch, got nil")
	}
}
