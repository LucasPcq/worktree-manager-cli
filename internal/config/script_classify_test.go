package config

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestClassifyScriptKind(t *testing.T) {
	cases := []struct {
		name string
		want domain.JobKind
	}{
		// Exact match
		{"dev", domain.JobKindService},
		{"start", domain.JobKindService},
		{"serve", domain.JobKindService},
		{"watch", domain.JobKindService},
		// Prefix (kw:*)
		{"dev:api", domain.JobKindService},
		{"start:web", domain.JobKindService},
		{"serve:storybook", domain.JobKindService},
		{"watch:types", domain.JobKindService},
		// Suffix (*:kw)
		{"api:dev", domain.JobKindService},
		{"web:start", domain.JobKindService},
		{"app:serve", domain.JobKindService},
		{"ts:watch", domain.JobKindService},
		// One-shot tasks
		{"build", domain.JobKindTask},
		{"test", domain.JobKindTask},
		{"lint", domain.JobKindTask},
		{"typecheck", domain.JobKindTask},
		{"deploy:prod", domain.JobKindTask},
		{"predev", domain.JobKindTask},   // no colon, not an exact match
		{"startup", domain.JobKindTask},  // no colon, not an exact match
		{"fresh-start", domain.JobKindTask}, // no colon
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyScriptKind(tc.name)
			if got != tc.want {
				t.Errorf("ClassifyScriptKind(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
