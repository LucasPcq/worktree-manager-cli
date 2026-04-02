// Package output formats and prints results. It contains zero decision logic.
package output

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatWorktreeListParams holds inputs for rendering the worktree list.
type FormatWorktreeListParams struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
}

// FormatWorktreeList renders a list of worktree statuses as an aligned table string.
func FormatWorktreeList(params FormatWorktreeListParams) string {
	if len(params.Statuses) == 0 {
		return "No worktrees found."
	}

	rows := buildRows(params.Statuses, params.ActiveBranch)
	widths := columnWidths(rows)

	var sb strings.Builder
	for _, row := range rows {
		line := formatRow(row, widths)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

type row struct {
	branch string
	tag    string
	status string
	ahead  string
}

func buildRows(statuses []domain.WorktreeStatus, activeBranch string) []row {
	rows := make([]row, 0, len(statuses))
	for _, s := range statuses {
		r := row{
			branch: styles.Bold.Render(s.Branch),
			tag:    formatTag(s.IsParent, s.Branch == activeBranch),
			status: formatDirtyStatus(s.IsDirty),
			ahead:  formatAhead(s.CommitsAhead),
		}
		rows = append(rows, r)
	}
	return rows
}

func formatTag(isParent bool, isActive bool) string {
	tags := ""
	if isParent {
		tags = styles.Muted.Render("(parent)")
	}
	if isActive {
		active := styles.Success.Render("● active")
		if tags != "" {
			tags += "  " + active
		} else {
			tags = active
		}
	}
	return tags
}

func formatDirtyStatus(dirty bool) string {
	if dirty {
		return styles.Warning.Render("dirty")
	}
	return styles.Success.Render("clean")
}

func formatAhead(count int) string {
	if count == 0 {
		return ""
	}
	if count == 1 {
		return styles.Muted.Render("1 commit ahead")
	}
	return styles.Muted.Render(fmt.Sprintf("%d commits ahead", count))
}

func columnWidths(rows []row) [4]int {
	var widths [4]int
	for _, r := range rows {
		widths[0] = max(widths[0], printableLen(r.branch))
		widths[1] = max(widths[1], printableLen(r.tag))
		widths[2] = max(widths[2], printableLen(r.status))
		widths[3] = max(widths[3], printableLen(r.ahead))
	}
	return widths
}

func formatRow(r row, widths [4]int) string {
	return fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
		widths[0]+ansiOverhead(r.branch), r.branch,
		widths[1]+ansiOverhead(r.tag), r.tag,
		widths[2]+ansiOverhead(r.status), r.status,
		r.ahead,
	)
}

// printableLen returns the length of a string without ANSI escape sequences.
func printableLen(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}

func ansiOverhead(s string) int {
	return len(s) - printableLen(s)
}
