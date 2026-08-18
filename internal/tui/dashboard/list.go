package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const (
	rowBar    = "▌"
	rowIndent = "  "
)

func (m Model) renderList(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:           layout.List,
		Title:          domain.DashboardListTitle,
		TitleRight:     m.addButton(layout),
		TitleRightZone: zoneAdd,
		Body:           m.listBody(layout),
		Zone:           zoneList,
	})
}

// addButton says out loud what it does when there is room for it to: it is the
// one way into the create flow that does not need the keyboard.
func (m Model) addButton(layout domain.DashboardLayout) string {
	room := layout.List.Width - borderWidth - paddingWidth - lipgloss.Width(domain.DashboardListTitle)
	label := domain.DashboardAddLabel
	if lipgloss.Width(domain.DashboardAddLabelLong)+buttonPadding+1 <= room {
		label = domain.DashboardAddLabelLong
	}
	return styles.DashboardAddButton.Render(label)
}

func (m Model) listBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if !m.loaded {
		return []string{styles.DashboardEmpty.Render(truncate(domain.LoadingWorktreesText, width))}
	}
	if len(m.statuses) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptyList, width))}
	}

	end := min(m.offset+layout.ListRows, len(m.statuses))
	rows := make([]string, 0, max(end-m.offset, 0)*(domain.DashboardRowHeight+domain.DashboardRowGap))
	for index := m.offset; index < end; index++ {
		if index > m.offset {
			rows = append(rows, "")
		}
		rows = append(rows, m.marks().Mark(rowZone(index), strings.Join(m.renderRow(index, width), "\n")))
	}
	return rows
}

// renderRow draws one worktree over two lines: what it is called and, under it,
// what its state amounts to. The selected one is tinted across its whole width
// and carries the accent bar, so the row the keyboard is on reads as a block
// rather than as a marker.
func (m Model) renderRow(index, width int) []string {
	status := m.statuses[index]
	inner := max(width-rowBarWidth, 0)
	pill := worktreepicker.BuildStatus(status)

	if index == m.cursor {
		bar := styles.DashboardRowBar.Render(rowBar + " ")
		name := spread(status.Branch, pill.Text, inner)
		meta := truncate(m.rowMeta(status, false), inner)
		return []string{
			bar + styles.DashboardRowSelected.Width(inner).Bold(true).Render(name),
			bar + styles.DashboardRowSelected.Width(inner).Render(meta),
		}
	}

	name := spread(styles.DashboardRowName.Render(status.Branch), pill.Render(), inner)
	meta := m.rowMeta(status, true)
	// Both lines are padded to the same width: the row is one clickable block, and
	// a short second line would cut its zone short.
	return []string{
		rowIndent + name,
		rowIndent + pad(truncateRendered(meta, inner), inner),
	}
}

// rowBarWidth is the gutter the accent bar and its space take, kept off the
// tinted span so the bar keeps its own color.
const rowBarWidth = 2

// rowMeta is the second line of a row: the same tags the pickers show, joined
// into one line rather than crowded against the branch name. A selected row is
// tinted as one span, so it takes them uncolored.
func (m Model) rowMeta(status domain.WorktreeStatus, colored bool) string {
	parts := make([]string, 0, 4)
	if parent := m.parents[status.Branch]; parent != "" {
		parts = append(parts, styleMeta(domain.DashboardMetaFromPrefix+parent, colored, styles.DashboardRowMeta))
	}
	for _, tag := range worktreepicker.BuildTags(worktreepicker.BuildTagsParams{Status: status, PRs: m.prs}) {
		if colored {
			parts = append(parts, tag.Render())
			continue
		}
		parts = append(parts, tag.Text)
	}
	if len(parts) == 0 {
		return styleMeta(domain.DashboardMetaNothing, colored, styles.DashboardRowMeta)
	}
	return strings.Join(parts, styleMeta(domain.DashboardMetaSeparator, colored, styles.DashboardRowMeta))
}

func styleMeta(text string, colored bool, style lipgloss.Style) string {
	if colored {
		return style.Render(text)
	}
	return text
}
