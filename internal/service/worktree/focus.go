package worktree

import (
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// FocusParams holds inputs for focusing a worktree.
// Deprecated: focus feature is removed; this stub exists for dashboard compatibility.
type FocusParams struct {
	ProjectDir string
	Branch     string
	Config     domain.Config
	Output     io.Writer
}

// Focus is a no-op stub retained for dashboard compilation.
// Deprecated: focus feature has been removed.
func Focus(_ FocusParams) error {
	return nil
}
