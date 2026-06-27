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

	// ErrUserAborted is returned when the user cancels an interactive prompt.
	ErrUserAborted = errors.New("user aborted")

	// ErrAborted signals a command that failed after already printing its own
	// report (e.g. a profile aborted by a failing task). The top-level handler
	// exits non-zero without printing a second, redundant error line.
	ErrAborted = errors.New("aborted")

	// ErrNotGitRepo is returned when the current directory is not a git repository.
	ErrNotGitRepo = errors.New("not a git repository")

	// ErrBranchNotFound is returned when a specified source branch does not exist locally.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrWorktreePathExists is returned when the target worktree directory already exists.
	ErrWorktreePathExists = errors.New("worktree path already exists")

	// ErrInvalidBasePath is returned when --to is not a usable repo-relative path
	// (blank/whitespace-only or absolute).
	ErrInvalidBasePath = errors.New("invalid base path: must be a non-empty path relative to the repo root")

	// ErrCannotCleanParent is returned when trying to clean the parent worktree.
	ErrCannotCleanParent = errors.New("cannot clean the parent worktree")

	// ErrGHNotInstalled is returned when the gh CLI is not found on PATH.
	ErrGHNotInstalled = errors.New("gh CLI not found — install it from https://cli.github.com")

	// ErrGHNotAuthenticated is returned when gh is not logged in to GitHub.
	ErrGHNotAuthenticated = errors.New("not logged in to GitHub — run 'gh auth login'")

	// ErrJobNotFound is returned when a referenced job is not declared in run.toml.
	ErrJobNotFound = errors.New("job not found")

	// ErrExtractConflict is returned when the selected changes do not apply
	// cleanly onto the target worktree. The extraction is aborted and the source
	// worktree is left untouched.
	ErrExtractConflict = errors.New("cannot apply the selected changes")

	// ErrNoChangesToExtract is returned when the source worktree has no
	// uncommitted changes to extract.
	ErrNoChangesToExtract = errors.New("no uncommitted changes to extract")

	// ErrNoFilesSelected is returned when the user selects no files to extract.
	ErrNoFilesSelected = errors.New("no files selected")

	// ErrSameWorktree is returned when the target worktree equals the source.
	ErrSameWorktree = errors.New("target worktree must differ from the source")

	// ErrReparentSelf is returned when a worktree is asked to become its own parent.
	ErrReparentSelf = errors.New("a worktree cannot be its own parent")
)
