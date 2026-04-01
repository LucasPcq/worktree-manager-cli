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
)
