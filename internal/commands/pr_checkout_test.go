package commands

import (
	"strings"
	"testing"

	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidatePRForCheckout_Valid(t *testing.T) {
	pr := domain.PRInfo{Number: 42, Branch: "feature/add-auth", IsFork: false}
	if err := ghservice.ValidatePRForCheckout(pr, []string{"main", "dev"}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidatePRForCheckout_Fork(t *testing.T) {
	pr := domain.PRInfo{Number: 42, Branch: "feature/add-auth", IsFork: true}
	err := ghservice.ValidatePRForCheckout(pr, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "LUC-40") {
		t.Errorf("expected error to mention LUC-40, got %q", err.Error())
	}
}

func TestValidatePRForCheckout_BranchExists(t *testing.T) {
	pr := domain.PRInfo{Number: 42, Branch: "feature/add-auth", IsFork: false}
	err := ghservice.ValidatePRForCheckout(pr, []string{"main", "feature/add-auth"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "feature/add-auth") {
		t.Errorf("expected error to mention branch name, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "wtm wt clean") {
		t.Errorf("expected error to mention cleanup hint, got %q", err.Error())
	}
}
