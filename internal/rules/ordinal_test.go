package rules

import "testing"

func TestAllocateOrdinal(t *testing.T) {
	cases := []struct {
		name  string
		taken []int
		want  int
	}{
		{"aucun worktree", nil, 1},
		{"que le principal", []int{0}, 1},
		{"contigu", []int{1, 2, 3}, 4},
		{"trou recyclé", []int{1, 3, 4}, 2},
		{"trou en tête", []int{2, 3}, 1},
		{"doublons", []int{1, 1, 2}, 3},
		{"désordonné", []int{5, 1, 4, 2}, 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AllocateOrdinal(c.taken); got != c.want {
				t.Errorf("AllocateOrdinal(%v) = %d, want %d", c.taken, got, c.want)
			}
		})
	}
}
