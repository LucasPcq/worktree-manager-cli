// Package dashboard implements the main TUI dashboard for wtm.
package dashboard

import "github.com/LucasPcq/wtm/internal/domain"

// worktreeListMsg is sent when worktree data has been loaded.
type worktreeListMsg struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
	Err          error
}

// logMsg updates the status bar at the bottom.
type logMsg string

// focusDoneMsg is sent when a focus operation completes.
type focusDoneMsg struct {
	Branch string
	Err    error
	Output string
}
