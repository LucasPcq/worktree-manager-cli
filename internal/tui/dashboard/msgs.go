// Package dashboard implements the main TUI dashboard for wtm.
package dashboard

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

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
}

// detailLoadedMsg is sent when worktree detail data has been loaded.
type detailLoadedMsg struct {
	Detail worktree.DetailResult
}

// hookOutputMsg streams a line of hook output to the log panel.
type hookOutputMsg struct {
	Line string
}

// actionDoneMsg signals that a suspend/resume action (new, clean) completed.
type actionDoneMsg struct {
	Err error
}

// serviceListMsg is sent when service statuses have been loaded from the daemon.
type serviceListMsg struct {
	Services []process.ServiceInfo
	Err      error
}

// servicesStartedMsg is sent after starting services via the daemon.
type servicesStartedMsg struct {
	Err error
}
