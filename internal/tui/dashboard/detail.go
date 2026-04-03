package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

type detailModel struct {
	detail    *worktree.DetailResult
	lastError string
	viewport  viewport.Model
	width     int
	height    int
	ready     bool
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

	d := m.detail

	// Header
	header := fmt.Sprintf("  %s\n  %s %s",
		styles.Bold.Render(d.Branch),
		styles.Muted.Render("Path:"),
		d.Path,
	)
	if d.SourceBranch != "" {
		header += fmt.Sprintf("\n  %s %s",
			styles.Muted.Render("From:"),
			d.SourceBranch,
		)
	}
	sections = append(sections, header)

	// Unpushed commits
	if d.UnpushedCommits > 0 {
		sections = append(sections,
			fmt.Sprintf("  %s %s",
				styles.Warning.Render(fmt.Sprintf("%d", d.UnpushedCommits)),
				"commits not pushed to remote",
			),
		)
	}

	// Context notes
	if d.ContextNotes != "" {
		sections = append(sections,
			"  "+styles.Muted.Render("Context notes")+"\n"+
				indentLines(d.ContextNotes, "  "),
		)
	}

	// Modified files
	sections = append(sections, renderModifiedFiles(d.ModifiedFiles))

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
		case "M":
			styledStatus = styles.Warning.Render(f.Status)
		case "D":
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

func indentLines(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
