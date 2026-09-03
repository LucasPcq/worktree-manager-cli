package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func (m Model) renderRunning(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.List,
		Title: domain.DashboardRunningTitle,
		Body:  m.runningBody(layout),
		Zone:  zoneRunning,
	})
}

// runningBlocks is what the daemon holds up, read off the poll: jobs and
// addresses, never a lazily loaded detail.
func (m Model) runningBlocks() []rules.RunWorktreeBlock {
	return rules.RunBoard(rules.RunBoardParams{
		Config:    m.runConfig,
		Jobs:      m.jobs,
		Addresses: m.addresses,
		Statuses:  m.statuses,
		Now:       time.Now(),
	})
}

func (m Model) selectedRunning() (rules.RunWorktreeBlock, bool) {
	blocks := m.runningBlocks()
	if m.runningCursor < 0 || m.runningCursor >= len(blocks) {
		return rules.RunWorktreeBlock{}, false
	}
	return blocks[m.runningCursor], true
}

// runningBody stacks one block per worktree: its name and count on a header
// row, then the same typed rows the detail panel's RUN section draws, so the
// two surfaces cannot read differently.
func (m Model) runningBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if width <= 0 {
		return nil
	}

	blocks := m.runningBlocks()
	if len(blocks) == 0 {
		return []string{
			styles.DashboardEmpty.Render(truncate(domain.DashboardRunningEmpty, width)),
			"",
			styles.DashboardRowMeta.Render(truncate(domain.DashboardRunningEmptyHint, width)),
		}
	}

	// The name column is sized across every block, not per block: the addresses
	// are what this tab is read down, and a column restarting at each worktree
	// would break that read.
	nameWidth := 0
	for _, block := range blocks {
		for _, row := range block.Rows {
			nameWidth = max(nameWidth, len([]rune(row.Key)))
		}
	}

	lines := make([]string, 0, len(blocks)*3)
	for index, block := range blocks {
		if index > 0 {
			lines = append(lines, "")
		}
		header := spread(
			m.runningBranchCell(index, block.Branch),
			styles.DashboardRowMeta.Render(fmt.Sprintf(domain.DashboardRunningUpFmt, block.Up)),
			width,
		)
		lines = append(lines, m.marks().Mark(runningRowZone(index), header))
		lines = append(lines, sectionRowLines(sectionRowLinesParams{
			Rows: block.Rows, Width: width, NameWidth: nameWidth,
		})...)
	}
	return lines
}

// The block under the cursor carries the accent bar and the tint a list row
// does, so the two tabs read as one dashboard.
func (m Model) runningBranchCell(index int, branch string) string {
	if index == m.runningCursor {
		return styles.DashboardRowBar.Render(rowBar+" ") + styles.DashboardRowSelected.Bold(true).Render(branch)
	}
	return strings.Repeat(" ", rowBarWidth) + styles.DashboardRowName.Render(branch)
}
