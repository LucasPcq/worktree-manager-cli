package rules

import (
	"testing"
	"time"
)

func TestRelativeAge(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"zéro", time.Time{}, ""},
		{"il y a 10 s", now.Add(-10 * time.Second), "just now"},
		{"il y a 5 min", now.Add(-5 * time.Minute), "5 min ago"},
		{"il y a 3 h", now.Add(-3 * time.Hour), "3 h ago"},
		{"il y a 2 j", now.Add(-48 * time.Hour), "2 d ago"},
		{"il y a 3 sem", now.Add(-21 * 24 * time.Hour), "3 w ago"},
		{"futur (horloge décalée)", now.Add(time.Hour), "just now"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RelativeAge(RelativeAgeParams{At: c.at, Now: now})
			if got != c.want {
				t.Errorf("RelativeAge(%v) = %q, want %q", c.at, got, c.want)
			}
		})
	}
}

func TestFetchIsStale(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		fetchedAt time.Time
		want      bool
	}{
		{"jamais fetché", time.Time{}, true},
		{"il y a 1 h", now.Add(-time.Hour), false},
		{"il y a 23 h", now.Add(-23 * time.Hour), false},
		{"il y a 25 h", now.Add(-25 * time.Hour), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FetchIsStale(FetchStalenessParams{FetchedAt: c.fetchedAt, Now: now})
			if got != c.want {
				t.Errorf("FetchIsStale(%v) = %v, want %v", c.fetchedAt, got, c.want)
			}
		})
	}
}
