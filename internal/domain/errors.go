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

	// ErrBranchNotFound is returned when a specified source or parent branch does
	// not exist as a local branch or an origin remote-tracking branch.
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

	// ErrRunNotInitialized is returned when a run command runs before the run
	// module is initialized — run.toml is absent or declares no job/profile. The
	// message points at the dedicated setup command.
	ErrRunNotInitialized = errors.New("run module not initialized — run `wtm run init` first")

	// ErrExtractConflict is returned when the selected changes do not apply
	// cleanly onto the target worktree. The extraction is aborted and the source
	// worktree is left untouched.
	ErrExtractConflict = errors.New("cannot apply the selected changes")

	// ErrNoChangesToExtract is returned when the source worktree has no
	// uncommitted changes to extract.
	ErrNoChangesToExtract = errors.New("no uncommitted changes to extract")

	// ErrNoDirtyWorktrees is returned when no managed worktree has uncommitted
	// changes, so the interactive source picker would be empty.
	ErrNoDirtyWorktrees = errors.New("no worktree has changes to extract")

	// ErrExtractSourceRequired is returned when extract is invoked without a
	// source worktree argument and cannot fall back to the interactive picker
	// (--yes, no terminal, or --output json).
	ErrExtractSourceRequired = errors.New("specify a source worktree (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrExtractFilesRequired is returned when extract cannot resolve which files
	// to extract: --files was omitted and there is no interactive picker (--yes, no
	// terminal, or --output json).
	ErrExtractFilesRequired = errors.New("specify --files (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrExtractTargetRequired is returned when extract cannot resolve the target
	// worktree: --to was omitted and there is no interactive picker (--yes, no
	// terminal, or --output json).
	ErrExtractTargetRequired = errors.New("specify --to (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrCleanBranchRequired is returned when clean is invoked without a worktree
	// branch argument and cannot fall back to the interactive picker (--yes, no
	// terminal, or --output json).
	ErrCleanBranchRequired = errors.New("specify a worktree branch to clean (no interactive picker under --yes, without a terminal, or in --output json mode)")

	// ErrNoFilesSelected is returned when the user selects no files to extract.
	ErrNoFilesSelected = errors.New("no files selected")

	// ErrSameWorktree is returned when the target worktree equals the source.
	ErrSameWorktree = errors.New("target worktree must differ from the source")

	// ErrReparentSelf is returned when a worktree is asked to become its own parent.
	ErrReparentSelf = errors.New("a worktree cannot be its own parent")
)
