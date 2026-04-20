package domain

import "testing"

func TestClassifyScriptKind(t *testing.T) {
	cases := []struct {
		name string
		want JobKind
	}{
		{"dev", JobKindService},
		{"start", JobKindService},
		{"serve", JobKindService},
		{"watch", JobKindService},
		{"dev:api", JobKindService},
		{"start:web", JobKindService},
		{"serve:storybook", JobKindService},
		{"watch:types", JobKindService},
		{"api:dev", JobKindService},
		{"web:start", JobKindService},
		{"app:serve", JobKindService},
		{"ts:watch", JobKindService},
		{"build", JobKindTask},
		{"test", JobKindTask},
		{"lint", JobKindTask},
		{"typecheck", JobKindTask},
		{"deploy:prod", JobKindTask},
		{"predev", JobKindTask},
		{"startup", JobKindTask},
		{"fresh-start", JobKindTask},
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
