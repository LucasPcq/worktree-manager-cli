package rules_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, domain.ExitCodeOK},
		{"generic", errors.New("boom"), domain.ExitCodeError},
		{"worktree path exists", domain.ErrWorktreePathExists, domain.ExitCodeWorktreeExists},
		{"worktree exists", domain.ErrWorktreeExists, domain.ExitCodeWorktreeExists},
		{"branch not found", domain.ErrBranchNotFound, domain.ExitCodeBranchNotFound},
		{"config not found", domain.ErrConfigNotFound, domain.ExitCodeConfigNotFound},
		{"pr exists", domain.ErrPRExists, domain.ExitCodePRExists},
		{"job not found", domain.ErrJobNotFound, domain.ExitCodeServiceNotFound},
		{"wrapped", fmt.Errorf("context: %w", domain.ErrBranchNotFound), domain.ExitCodeBranchNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
