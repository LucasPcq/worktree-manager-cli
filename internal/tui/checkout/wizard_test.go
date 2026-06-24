package checkout

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"shorter than max", "abc", 10, "abc"},
		{"equal to max", "abcde", 5, "abcde"},
		{"longer than max", "abcdef", 5, "abcd…"},
		{"empty", "", 5, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := truncate(c.in, c.maxLen); got != c.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.maxLen, got, c.want)
			}
		})
	}
}

func TestBuildPRItems_LinkedAndForkDisabled(t *testing.T) {
	prs := []domain.PRInfo{
		{Number: 1, Title: "open one", Author: "alice", Branch: "feat/a"},
		{Number: 2, Title: "already checked out", Author: "bob", Branch: "feat/b"},
		{Number: 3, Title: "from a fork", Author: "carol", Branch: "feat/c", IsFork: true},
	}
	existing := []string{"feat/b"}

	items := buildPRItems(prs, existing)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if items[0].Disabled || len(items[0].Badges) != 0 {
		t.Errorf("PR #1 should be selectable with no badge, got %+v", items[0])
	}
	if !items[1].Disabled || items[1].Badges[0].Text != domain.BadgeTextLinked {
		t.Errorf("PR #2 should be disabled with 'linked' badge, got %+v", items[1])
	}
	if !items[2].Disabled || items[2].Badges[0].Text != domain.BadgeTextFork {
		t.Errorf("PR #3 should be disabled with 'fork' badge, got %+v", items[2])
	}
}

func TestBuildBranchItems_BaseFirst(t *testing.T) {
	items := buildBranchItems([]string{"zeta", "alpha", "main"}, "main")

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Value != "main" {
		t.Errorf("base branch should be first, got %q", items[0].Value)
	}
	if items[1].Value != "alpha" || items[2].Value != "zeta" {
		t.Errorf("rest should be sorted, got %q then %q", items[1].Value, items[2].Value)
	}
}
