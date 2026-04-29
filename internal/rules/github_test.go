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

func TestBranchTitleFromName(t *testing.T) {
	cases := []struct{ branch, want string }{
		{"feature/add-auth", "Add auth"},
		{"fix/LUC-42-login-bug", "Login bug"},
		{"my-branch", "My branch"},
		{"", ""},
	}
	for _, c := range cases {
		if got := BranchTitleFromName(c.branch); got != c.want {
			t.Errorf("BranchTitleFromName(%q) = %q, want %q", c.branch, got, c.want)
		}
	}
}

func TestExtractPRNumber(t *testing.T) {
	n, err := ExtractPRNumber("https://github.com/owner/repo/pull/42")
	if err != nil || n != 42 {
		t.Errorf("expected 42, got %d, err %v", n, err)
	}
	_, err = ExtractPRNumber("https://github.com/owner/repo/issues/1")
	if err == nil {
		t.Error("expected error for non-PR URL")
	}
}
