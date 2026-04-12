package domain

// HookEvent identifies when a hook should fire.
type HookEvent string

const (
	// HookOnCreate fires after a new worktree is created.
	HookOnCreate HookEvent = "on_create"
)

// HookCommand represents a hook entry — either a simple command string or a {cmd, cwd} object.
type HookCommand struct {
	Cmd             string `toml:"cmd"`
	Cwd             string `toml:"cwd"`
	ContinueOnError bool   `toml:"continue_on_error"`
}

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
