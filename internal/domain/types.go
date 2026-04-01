package domain

// Worktree represents a git worktree managed by wtm.
type Worktree struct {
	Name   string
	Path   string
	Branch string
}

// HookEvent identifies when a hook should fire.
type HookEvent string

const (
	// HookOnCreate fires after a new worktree is created.
	HookOnCreate HookEvent = "on_create"

	// HookOnFocus fires when switching to a worktree.
	HookOnFocus HookEvent = "on_focus"

	// HookOnBlur fires when leaving a worktree.
	HookOnBlur HookEvent = "on_blur"
)

// EnvStrategy determines how .env files are provisioned in new worktrees.
type EnvStrategy string

const (
	// EnvStrategyExample copies .env.example.
	EnvStrategyExample EnvStrategy = "example"

	// EnvStrategyMain copies .env from the main worktree.
	EnvStrategyMain EnvStrategy = "main"

	// EnvStrategyParent copies .env from the parent worktree.
	EnvStrategyParent EnvStrategy = "parent"
)
