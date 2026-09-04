package runview

import (
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// sidebarRow is one line of the job list. Above several worktrees the list is
// two levels deep — a heading, then that worktree's jobs — rather than flat
// with a worktree column: the sidebar is narrow, and a column would be the
// first thing truncated. Exactly one of the two fields is set.
type sidebarRow struct {
	Header string
	View   runlogs.JobView
}

// rows is the job list as the sidebar draws it. Headings are not selectable and
// never appear below a single worktree, where naming it would only repeat what
// the command was told.
func (m Model) rows() []sidebarRow {
	visible := m.visible()
	if !m.multi() {
		rows := make([]sidebarRow, 0, len(visible))
		for _, view := range visible {
			rows = append(rows, sidebarRow{View: view})
		}
		return rows
	}

	rows := make([]sidebarRow, 0, len(visible)+2)
	current := ""
	for _, view := range visible {
		if view.WorkDir != current {
			current = view.WorkDir
			rows = append(rows, sidebarRow{Header: headingOf(view)})
		}
		rows = append(rows, sidebarRow{View: view})
	}
	return rows
}

// headingOf names a worktree, falling back to its path for one git could not
// name: a heading nobody can read is worse than a long one.
func headingOf(view runlogs.JobView) string {
	if view.Worktree != "" {
		return view.Worktree
	}
	return view.WorkDir
}

// selectedRowIndex places the cursor among the rows, headings included, which
// is what the sidebar's scroll offset is measured against.
func selectedRowIndex(rows []sidebarRow, selected jobKey) int {
	for index, row := range rows {
		if row.Header == "" && viewKey(row.View) == selected {
			return index
		}
	}
	return 0
}

// indent sets a job's rows in from the heading above them, and leaves them
// where they were when there is no heading.
func (m Model) indent() string {
	if !m.multi() {
		return ""
	}
	return " "
}
