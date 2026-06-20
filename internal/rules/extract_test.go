package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestClassifyExtractStatus(t *testing.T) {
	cases := []struct {
		status string
		want   domain.ExtractFileStatus
	}{
		{"??", domain.ExtractStatusUntracked},
		{"M", domain.ExtractStatusModified},
		{"MM", domain.ExtractStatusModified},
		{"A", domain.ExtractStatusModified},
		{"D", domain.ExtractStatusDeleted},
		{"AD", domain.ExtractStatusDeleted},
	}

	for _, c := range cases {
		if got := rules.ClassifyExtractStatus(c.status); got != c.want {
			t.Errorf("ClassifyExtractStatus(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}

func TestExtractStatusLabel(t *testing.T) {
	cases := map[domain.ExtractFileStatus]string{
		domain.ExtractStatusUntracked: "new",
		domain.ExtractStatusDeleted:   "del",
		domain.ExtractStatusModified:  "mod",
	}
	for status, want := range cases {
		if got := rules.ExtractStatusLabel(status); got != want {
			t.Errorf("ExtractStatusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}
