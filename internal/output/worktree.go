// Package output formats and prints results. It contains zero decision logic.
package output

import (
	"encoding/json"
	"fmt"
	"io"
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
	Services     []process.JobInfo
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

func buildRows(statuses []domain.WorktreeStatus, activeBranch string, prs []domain.PRInfo, svcs []process.JobInfo) []row {
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

func formatServicesTag(worktreePath string, svcs []process.JobInfo) string {
	for _, svc := range svcs {
		if svc.WorkDir == worktreePath && svc.Status == domain.JobStatusRunning {
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

// WriteWorktreeListJSONParams holds inputs for the JSON list serializer.
type WriteWorktreeListJSONParams struct {
	Statuses []domain.WorktreeStatus
	PRInfos  []domain.PRInfo
	Services []process.JobInfo
}

// WriteWorktreeListJSON writes a JSON array describing each worktree.
func WriteWorktreeListJSON(w io.Writer, params WriteWorktreeListJSONParams) error {
	entries := make([]domain.WorktreeListEntry, 0, len(params.Statuses))
	for _, s := range params.Statuses {
		entries = append(entries, domain.WorktreeListEntry{
			Branch:       s.Branch,
			Path:         s.Path,
			IsParent:     s.IsParent,
			IsDirty:      s.IsDirty,
			CommitsAhead: s.CommitsAhead,
			CreatedAt:    s.CreatedAt,
			PR:           matchPR(s.Branch, params.PRInfos),
			Services:     matchRunningServices(s.Path, params.Services),
		})
	}
	return encodeJSON(w, entries)
}

func matchPR(branch string, prs []domain.PRInfo) *domain.WorktreeListPR {
	for _, pr := range prs {
		if pr.Branch == branch {
			return &domain.WorktreeListPR{Number: pr.Number, URL: pr.URL, State: pr.State}
		}
	}
	return nil
}

func matchRunningServices(worktreePath string, services []process.JobInfo) []string {
	names := make([]string, 0)
	for _, svc := range services {
		if svc.WorkDir == worktreePath && svc.Status == domain.JobStatusRunning {
			names = append(names, svc.Name)
		}
	}
	return names
}

// WriteWorktreeCreateJSON writes the JSON payload for `wt create`.
func WriteWorktreeCreateJSON(w io.Writer, v any) error {
	return encodeJSON(w, v)
}

// WriteWorktreeCleanJSONParams holds inputs for the clean payload.
type WriteWorktreeCleanJSONParams struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// WriteWorktreeCleanJSON writes the JSON payload for `wt clean`.
func WriteWorktreeCleanJSON(w io.Writer, params WriteWorktreeCleanJSONParams) error {
	return encodeJSON(w, params)
}

// encodeJSON writes v as indented JSON to w.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
