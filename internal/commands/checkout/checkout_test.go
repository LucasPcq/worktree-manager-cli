package checkout

import "testing"

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
			if got := firstNonEmpty(c.values...); got != c.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", c.values, got, c.want)
			}
		})
	}
}
