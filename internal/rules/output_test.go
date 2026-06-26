package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestIsHumanFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{domain.OutputText, true},
		{"", true},
		{"table", true},
		{domain.OutputJSON, false},
	}
	for _, tt := range tests {
		if got := IsHumanFormat(tt.format); got != tt.want {
			t.Errorf("IsHumanFormat(%q) = %v, want %v", tt.format, got, tt.want)
		}
	}
}
