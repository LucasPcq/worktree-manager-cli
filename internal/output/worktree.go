// Package output formats and prints results. It contains zero decision logic.
package output

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatWorktreeListParams holds inputs for rendering the worktree list.
type FormatWorktreeListParams struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
	PRInfos      []domain.PRInfo
	Services     []process.ServiceInfo
}

// FormatWorktreeList renders a list of worktree statuses as an aligned table string.
func FormatWorktreeList(params FormatWorktreeListParams) string {
	if len(params.Statuses) == 0 {
		return "No worktrees found."
	}

	rows := buildRows(params.Statuses, params.ActiveBranch, params.PRInfos, params.Services)
	widths := columnWidths(rows)

	var builder strings.Builder
	for _, row := range rows {
		line := formatRow(row, widths)
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

type row struct {
	branch   string
	tag      string
	pr       string
	services string
	status   string
	ahead    string
}

func buildRows(statuses []domain.WorktreeStatus, activeBranch string, prs []domain.PRInfo, svcs []process.ServiceInfo) []row {
	rows := make([]row, 0, len(statuses))
	for _, s := range statuses {
		r := row{
			branch:   styles.Bold.Render(s.Branch),
			tag:      formatTag(s.IsParent, s.Branch == activeBranch),
			pr:       formatPRTag(s.Branch, prs),
			services: formatServicesTag(s.Path, svcs),
			status:   formatDirtyStatus(s.IsDirty),
			ahead:    formatAhead(s.CommitsAhead),
		}
		rows = append(rows, r)
	}
	return rows
}

func formatPRTag(branch string, prs []domain.PRInfo) string {
	for _, pr := range prs {
		if pr.Branch == branch {
			return styles.Success.Render(fmt.Sprintf("PR #%d", pr.Number))
		}
	}
	return ""
}

func formatServicesTag(worktreePath string, svcs []process.ServiceInfo) string {
	for _, svc := range svcs {
		if svc.WorkDir == worktreePath && svc.Status == domain.ServiceStatusRunning {
			return styles.Success.Render("services")
		}
	}
	return ""
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

func columnWidths(rows []row) [6]int {
	var widths [6]int
	for _, r := range rows {
		widths[0] = max(widths[0], printableLen(r.branch))
		widths[1] = max(widths[1], printableLen(r.tag))
		widths[2] = max(widths[2], printableLen(r.pr))
		widths[3] = max(widths[3], printableLen(r.services))
		widths[4] = max(widths[4], printableLen(r.status))
		widths[5] = max(widths[5], printableLen(r.ahead))
	}
	return widths
}

func formatRow(r row, widths [6]int) string {
	return fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s  %s",
		widths[0]+ansiOverhead(r.branch), r.branch,
		widths[1]+ansiOverhead(r.tag), r.tag,
		widths[2]+ansiOverhead(r.pr), r.pr,
		widths[3]+ansiOverhead(r.services), r.services,
		widths[4]+ansiOverhead(r.status), r.status,
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
