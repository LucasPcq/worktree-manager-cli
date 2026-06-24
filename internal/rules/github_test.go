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

func TestPRFilterFor(t *testing.T) {
	cases := []struct {
		name   string
		params PRFilterParams
		want   domain.PRFilter
	}{
		{"none", PRFilterParams{}, domain.PRFilterAll},
		{"mine", PRFilterParams{Mine: true}, domain.PRFilterMine},
		{"review", PRFilterParams{Review: true}, domain.PRFilterReviewRequested},
		{"review wins over mine", PRFilterParams{Review: true, Mine: true}, domain.PRFilterReviewRequested},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PRFilterFor(c.params); got != c.want {
				t.Errorf("PRFilterFor(%+v) = %q, want %q", c.params, got, c.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"first wins", []string{"a", "b"}, "a"},
		{"skip empty", []string{"", "b", "c"}, "b"},
		{"all empty", []string{"", ""}, ""},
		{"none", nil, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FirstNonEmpty(c.values...); got != c.want {
				t.Errorf("FirstNonEmpty(%v) = %q, want %q", c.values, got, c.want)
			}
		})
	}
}
