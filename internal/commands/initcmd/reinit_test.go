package initcmd

import (
	"strings"
	"testing"
)

func TestParseSections(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
	}{
		{name: "single", input: []string{"services"}, want: []string{"services"}},
		{name: "worktrees", input: []string{"worktrees"}, want: []string{"worktrees"}},
		{name: "csv", input: []string{"env,services"}, want: []string{"env", "services"}},
		{name: "repeated", input: []string{"env", "hooks"}, want: []string{"env", "hooks"}},
		{name: "dedup", input: []string{"env", "env"}, want: []string{"env"}},
		{name: "trim", input: []string{" env , services "}, want: []string{"env", "services"}},
		{name: "unknown", input: []string{"nope"}, wantErr: true},
		{name: "mixed valid+invalid", input: []string{"env,bogus"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSections(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %v", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

