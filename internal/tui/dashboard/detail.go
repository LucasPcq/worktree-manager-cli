package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

type detailModel struct {
	detail          *worktree.DetailResult
	serviceStatuses []process.ServiceInfo
	lastError string
	viewport  viewport.Model
	width     int
	height    int
}

func (m *detailModel) setSize(width int, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
}

func (m *detailModel) setDetail(detail *worktree.DetailResult) {
	m.detail = detail
	m.lastError = ""
	m.refreshContent()
}

// setContent sets arbitrary rendered content (e.g. PR detail) in the viewport.
func (m *detailModel) setContent(content string) {
	m.detail = nil
	m.lastError = ""
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

func (m *detailModel) refreshContent() {
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
}

func (m detailModel) renderContent() string {
	var sections []string

	if m.lastError != "" {
		sections = append(sections,
			styles.Warning.Render("  Last error:")+"\n"+
				styles.Muted.Render("  "+m.lastError),
		)
	}

	if m.detail == nil {
		if len(sections) == 0 {
			return styles.Muted.Render("  Select a worktree to view details")
		}
		return strings.Join(sections, "\n\n")
	}

	detail := m.detail

	// Header
	header := fmt.Sprintf("  %s\n  %s %s",
		styles.Bold.Render(detail.Branch),
		styles.Muted.Render("Path:"),
		detail.Path,
	)
	if detail.SourceBranch != "" {
		header += fmt.Sprintf("\n  %s %s",
			styles.Muted.Render("From:"),
			detail.SourceBranch,
		)
	}
	sections = append(sections, header)

	// Services — only show services running in this worktree
	worktreeServices := filterServicesByWorkDir(m.serviceStatuses, detail.Path)
	if len(worktreeServices) > 0 {
		sections = append(sections, renderServiceStatuses(worktreeServices))
	}

	// Unpushed commits
	if detail.UnpushedCommits > 0 {
		sections = append(sections,
			fmt.Sprintf("  %s %s",
				styles.Warning.Render(fmt.Sprintf("%d", detail.UnpushedCommits)),
				"commits not pushed to remote",
			),
		)
	}

	// Context notes
	if detail.ContextNotes != "" {
		sections = append(sections,
			"  "+styles.Muted.Render("Context notes")+"\n"+
				indentLines(detail.ContextNotes, "  "),
		)
	}

	// Modified files
	sections = append(sections, renderModifiedFiles(detail.ModifiedFiles))

	return strings.Join(sections, "\n\n")
}

func renderModifiedFiles(files []infra.ModifiedFile) string {
	title := "  " + styles.Muted.Render("Modified files")

	if len(files) == 0 {
		return title + "\n" + styles.Muted.Render("  No modified files")
	}

	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("\n")

	for _, f := range files {
		var styledStatus string
		switch f.Status {
		case "M", "D":
			styledStatus = styles.Warning.Render(f.Status)
		case "A":
			styledStatus = styles.Success.Render(f.Status)
		default:
			styledStatus = styles.Muted.Render(f.Status)
		}
		builder.WriteString(fmt.Sprintf("  %s %s\n", styledStatus, f.Path))
	}

	return builder.String()
}

func filterServicesByWorkDir(services []process.ServiceInfo, workDir string) []process.ServiceInfo {
	var filtered []process.ServiceInfo
	for _, svc := range services {
		if svc.WorkDir == workDir {
			filtered = append(filtered, svc)
		}
	}
	return filtered
}

func renderServiceStatuses(services []process.ServiceInfo) string {
	title := "  " + styles.Muted.Render("Services")
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("\n")

	for _, svc := range services {
		var indicator string
		switch svc.Status {
		case domain.ServiceStatusRunning:
			indicator = styles.ActiveIndicator.Render("●")
		case domain.ServiceStatusCrashed:
			indicator = styles.DirtyIndicator.Render("✗")
		default:
			indicator = styles.Muted.Render("○")
		}
		builder.WriteString(fmt.Sprintf("  %s %s  %s  (pid %d)\n",
			indicator, svc.Name, styles.Muted.Render(string(svc.Status)), svc.PID))
	}

	return builder.String()
}

func indentLines(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
