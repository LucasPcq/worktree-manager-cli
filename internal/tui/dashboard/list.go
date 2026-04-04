package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
)

type listModel struct {
	items           []domain.WorktreeStatus
	activeBranch    string
	serviceStatuses []process.ServiceInfo
	cursor          int
	width           int
	height          int
}

func (m *listModel) setItems(statuses []domain.WorktreeStatus, activeBranch string) {
	m.items = statuses
	m.activeBranch = activeBranch
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func (m *listModel) setSize(width int, height int) {
	m.width = width
	m.height = height
}

func (m *listModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *listModel) moveDown() {
	if m.cursor < len(m.items)-1 {
		m.cursor++
	}
}

func (m listModel) selectedStatus() (domain.WorktreeStatus, bool) {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return domain.WorktreeStatus{}, false
	}
	return m.items[m.cursor], true
}

func (m listModel) view(focused bool) string {
	if len(m.items) == 0 {
		return styles.NormalItem.Render("  No worktrees found.")
	}

	var builder strings.Builder
	for i, item := range m.items {
		selected := i == m.cursor && focused
		builder.WriteString(m.renderLine(item, selected))
		builder.WriteString("\n")
	}

	return builder.String()
}

func (m listModel) hasRunningServices(worktreePath string) bool {
	for _, svc := range m.serviceStatuses {
		if svc.WorkDir == worktreePath && svc.Status == domain.ServiceStatusRunning {
			return true
		}
	}
	return false
}

func (m listModel) renderLine(item domain.WorktreeStatus, selected bool) string {
	// Cursor
	cursor := "  "
	if selected {
		cursor = styles.Primary.Render("▸ ")
	}

	// Branch name
	branch := item.Branch

	// Right-side tags
	var tags []string

	if item.CommitsAhead > 0 {
		tags = append(tags, styles.Muted.Render(fmt.Sprintf("%d ahead", item.CommitsAhead)))
	}

	if item.IsParent {
		tags = append(tags, styles.Muted.Render("parent"))
	}

	if item.IsDirty {
		tags = append(tags, styles.DirtyIndicator.Render("dirty"))
	} else {
		tags = append(tags, styles.CleanIndicator.Render("clean"))
	}

	// Services tag
	if m.hasRunningServices(item.Path) {
		tags = append(tags, styles.ActiveIndicator.Render("services"))
	} else {
		tags = append(tags, styles.Muted.Render("services"))
	}

	// Focus tag
	if item.Branch == m.activeBranch {
		tags = append(tags, styles.ActiveIndicator.Render("focus"))
	} else {
		tags = append(tags, styles.Muted.Render("focus"))
	}

	rightSide := strings.Join(tags, "  ")
	rightLen := printableWidth(rightSide)

	// Truncate branch if needed
	maxBranchLen := max(8, m.width-rightLen-6)
	if len(branch) > maxBranchLen {
		branch = branch[:maxBranchLen-1] + "…"
	}

	// Branch style
	if selected {
		branch = styles.Bold.Render(branch)
	}

	leftSide := cursor + branch
	leftLen := printableWidth(leftSide)
	gap := max(1, m.width-leftLen-rightLen)

	return leftSide + strings.Repeat(" ", gap) + rightSide
}
