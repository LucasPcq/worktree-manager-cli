package wt

import "testing"

func TestSyncIncludesBase(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
		base     string
		want     bool
	}{
		{"nil selection means all", nil, "main", true},
		{"base explicitly selected", []string{"feat", "main"}, "main", true},
		{"base not selected", []string{"feat"}, "main", false},
		{"empty non-nil selection", []string{}, "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := syncIncludesBase(tt.selected, tt.base); got != tt.want {
				t.Fatalf("syncIncludesBase(%v, %q) = %v, want %v", tt.selected, tt.base, got, tt.want)
			}
		})
	}
}
