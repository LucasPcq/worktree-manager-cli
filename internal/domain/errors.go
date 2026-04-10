package domain

import "errors"

var (
	// ErrWorktreeNotFound is returned when a worktree cannot be located.
	ErrWorktreeNotFound = errors.New("worktree not found")

	// ErrWorktreeExists is returned when attempting to create a duplicate worktree.
	ErrWorktreeExists = errors.New("worktree already exists")

	// ErrConfigNotFound is returned when no configuration file is found.
	ErrConfigNotFound = errors.New("config file not found")

	// ErrInvalidConfig is returned when configuration fails validation.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrInvalidEnvStrategy is returned when env.strategy has an unknown value.
	ErrInvalidEnvStrategy = errors.New("invalid env strategy: must be example, main, or parent")

	// ErrInvalidShellType is returned when shell has an unknown value.
	ErrInvalidShellType = errors.New("invalid shell type: must be zsh, bash, or fish")

	// ErrInvalidAgentType is returned when agent has an unknown value.
	ErrInvalidAgentType = errors.New("invalid agent type: must be claude-code, cursor, or none")

	// ErrUserAborted is returned when the user cancels an interactive prompt.
	ErrUserAborted = errors.New("user aborted")

	// ErrNotGitRepo is returned when the current directory is not a git repository.
	ErrNotGitRepo = errors.New("not a git repository")

	// ErrBranchNotFound is returned when a specified source branch does not exist locally.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrWorktreePathExists is returned when the target worktree directory already exists.
	ErrWorktreePathExists = errors.New("worktree path already exists")

	// ErrCannotCleanParent is returned when trying to clean the parent worktree.
	ErrCannotCleanParent = errors.New("cannot clean the parent worktree")

	// ErrAuthNotConfigured is returned when no GitHub token is stored.
	ErrAuthNotConfigured = errors.New("not authenticated — run `wtm auth login`")

	// ErrAuthAlreadyLoggedIn is returned when attempting to login while already authenticated.
	ErrAuthAlreadyLoggedIn = errors.New("already logged in")

	// ErrAuthDeviceFlowTimeout is returned when the device flow polling expires.
	ErrAuthDeviceFlowTimeout = errors.New("authentication timed out")

	// ErrAuthNeedsReauth is returned when the stored access token is expired
	// and cannot be refreshed (refresh token missing or itself expired).
	ErrAuthNeedsReauth = errors.New("authentication expired — run `wtm auth login` again")
)
