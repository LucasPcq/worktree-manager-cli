package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestClassifyScriptKind(t *testing.T) {
	cases := []struct {
		name string
		want domain.JobKind
	}{
		{"dev", domain.JobKindService},
		{"start", domain.JobKindService},
		{"serve", domain.JobKindService},
		{"watch", domain.JobKindService},
		{"dev:api", domain.JobKindService},
		{"start:web", domain.JobKindService},
		{"serve:storybook", domain.JobKindService},
		{"watch:types", domain.JobKindService},
		{"api:dev", domain.JobKindService},
		{"web:start", domain.JobKindService},
		{"app:serve", domain.JobKindService},
		{"ts:watch", domain.JobKindService},
		{"build", domain.JobKindTask},
		{"test", domain.JobKindTask},
		{"lint", domain.JobKindTask},
		{"typecheck", domain.JobKindTask},
		{"deploy:prod", domain.JobKindTask},
		{"predev", domain.JobKindTask},
		{"startup", domain.JobKindTask},
		{"fresh-start", domain.JobKindTask},
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
