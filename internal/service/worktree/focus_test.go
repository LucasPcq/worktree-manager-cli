package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/state"
)

func TestRunBlurIfActive_EmptyState(t *testing.T) {
	current := state.State{ActiveWorktreePath: ""}
	cfg := domain.Config{
		Project: domain.ProjectConfig{
			Hooks: domain.HooksConfig{
				OnBlur: []domain.HookCommand{
					{Cmd: "echo blur"},
				},
			},
		},
	}

	err := runBlurIfActive(current, cfg)
	if err != nil {
		t.Errorf("expected nil error for empty state, got: %v", err)
	}
}

func TestRunBlurIfActive_ActiveStateNoHooks(t *testing.T) {
	current := state.State{
		ActiveWorktree:     "feature/test",
		ActiveWorktreePath: "/tmp/some-worktree",
	}
	cfg := domain.Config{
		Project: domain.ProjectConfig{
			Hooks: domain.HooksConfig{
				OnBlur: []domain.HookCommand{},
			},
		},
	}

	err := runBlurIfActive(current, cfg)
	if err != nil {
		t.Errorf("expected nil error when no blur hooks configured, got: %v", err)
	}
}
