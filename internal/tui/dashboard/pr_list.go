package dashboard

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

type prListModel struct {
	items          []domain.PRInfo
	worktreePaths  map[string]bool // branch → has local worktree
	filter         domain.PRFilter
	cursor         int
	width          int
	height         int
	loading        bool
	lastError      string
}

func newPRListModel() prListModel {
	return prListModel{
		filter:  domain.PRFilterAll,
		loading: true,
	}
}

func (m *prListModel) setItems(prs []domain.PRInfo) {
	m.items = prs
	m.loading = false
	m.lastError = ""
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func (m *prListModel) setError(err error) {
	m.loading = false
	m.lastError = err.Error()
	m.items = nil
}

func (m *prListModel) setWorktreeBranches(statuses []domain.WorktreeStatus) {
	m.worktreePaths = make(map[string]bool)
	for _, s := range statuses {
		m.worktreePaths[s.Branch] = true
	}
}

func (m *prListModel) setSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *prListModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *prListModel) moveDown() {
	if m.cursor < len(m.items)-1 {
		m.cursor++
	}
}

func (m *prListModel) selectedPR() (domain.PRInfo, bool) {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return domain.PRInfo{}, false
	}
	return m.items[m.cursor], true
}

func (m *prListModel) cycleFilter() {
	switch m.filter {
	case domain.PRFilterAll:
		m.filter = domain.PRFilterReviewRequested
	case domain.PRFilterReviewRequested:
		m.filter = domain.PRFilterMine
	case domain.PRFilterMine:
		m.filter = domain.PRFilterAll
	}
	m.loading = true
	m.cursor = 0
}

func (m prListModel) filterLabel() string {
	switch m.filter {
	case domain.PRFilterReviewRequested:
		return "Review requested"
	case domain.PRFilterMine:
		return "My PRs"
	default:
		return "All PRs"
	}
}

func (m prListModel) view(focused bool) string {
	if m.loading {
		return styles.Muted.Render("  Loading PRs...")
	}

	if m.lastError != "" {
		return styles.Warning.Render("  " + m.lastError)
	}

	if len(m.items) == 0 {
		return styles.Muted.Render("  No pull requests.")
	}

	var b strings.Builder
	for i, pr := range m.items {
		selected := i == m.cursor && focused
		b.WriteString(m.renderLine(pr, selected))
		b.WriteString("\n")
	}
	return b.String()
}

func (m prListModel) renderLine(pr domain.PRInfo, selected bool) string {
	cursor := "  "
	if selected {
		cursor = styles.Primary.Render("▸ ")
	}

	number := styles.Muted.Render(fmt.Sprintf("#%d", pr.Number))

	title := pr.Title
	maxTitleLen := max(10, m.width-30)
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-1] + "…"
	}
	if selected {
		title = styles.Bold.Render(title)
	}

	var tags []string

	tags = append(tags, styles.Muted.Render(prAge(pr.CreatedAt)))

	if pr.Draft {
		tags = append(tags, styles.Muted.Render("draft"))
	}

	// Local worktree indicator
	if m.worktreePaths[pr.Branch] {
		tags = append(tags, styles.ActiveIndicator.Render("local"))
	}

	rightSide := strings.Join(tags, "  ")
	rightLen := printableWidth(rightSide)

	leftSide := cursor + number + " " + title
	leftLen := printableWidth(leftSide)
	gap := max(1, m.width-leftLen-rightLen)

	return leftSide + strings.Repeat(" ", gap) + rightSide
}

func prAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(math.Floor(d.Hours()/24)))
	}
}
