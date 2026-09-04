package runlogs

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewMergedBoard shows several worktrees' jobs as one board, in the order they
// were selected. A board of one is returned as itself: a surface reading a
// single worktree must not pay for a routing layer it never uses.
func NewMergedBoard(boards []Board) Board {
	if len(boards) == 1 {
		return boards[0]
	}
	return &mergedBoard{boards: boards}
}

type mergedBoard struct {
	boards []Board
}

// Refresh refreshes every board and reports the first failure, having asked
// them all: a daemon that answered for two worktrees out of three still has
// something to show for the two.
func (m *mergedBoard) Refresh() error {
	var first error
	for _, board := range m.boards {
		if err := board.Refresh(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *mergedBoard) Jobs() []JobView {
	var views []JobView
	for _, board := range m.boards {
		views = append(views, board.Jobs()...)
	}
	return views
}

func (m *mergedBoard) Attach(params AttachParams) (Stream, error) {
	board, err := m.route(params.WorkDir)
	if err != nil {
		return nil, err
	}
	return board.Attach(params)
}

func (m *mergedBoard) History(params HistoryParams) ([]string, error) {
	board, err := m.route(params.WorkDir)
	if err != nil {
		return nil, err
	}
	return board.History(params)
}

// route finds the board holding a worktree. An unnamed worktree is refused
// rather than defaulted to the first: guessing would read one job's output
// under another's name.
func (m *mergedBoard) route(workDir string) (Board, error) {
	for _, board := range m.boards {
		for _, view := range board.Jobs() {
			if view.WorkDir == workDir {
				return board, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, workDir)
}
