package dashboard

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const detailLabelWidth = 9

func (m Model) renderDetail(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.Detail,
		Title: domain.DashboardDetailTitle,
		Body:  m.detailBody(layout),
		Zone:  zoneDetail,
	})
}

func (m Model) detailBody(layout domain.DashboardLayout) []string {
	width := layout.Detail.Width - borderWidth - paddingWidth
	status, ok := m.selected()
	if !ok {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptySelection, width))}
	}

	fields := [][2]string{
		{domain.DashboardLabelPath, status.Path},
		{domain.DashboardLabelParent, m.parentOf(status)},
		{domain.DashboardLabelState, stateText(status)},
		{domain.DashboardLabelBase, baseText(status)},
		{domain.DashboardLabelOrigin, originText(status)},
	}
	if status.RebaseInProgress {
		fields = append(fields, [2]string{domain.DashboardLabelRebase, domain.DashboardRebaseInProgress})
	}
	fields = append(fields,
		[2]string{domain.DashboardLabelPR, m.prText(status.Branch)},
		[2]string{domain.DashboardLabelCreated, createdText(status)},
	)

	lines := []string{styles.DashboardBranch.Render(truncate(status.Branch, width)), ""}
	for _, field := range fields {
		label := styles.DashboardLabel.Render(pad(field[0], detailLabelWidth))
		lines = append(lines, label+styles.DashboardValue.Render(truncate(field[1], max(width-detailLabelWidth, 0))))
	}
	return lines
}

func (m Model) parentOf(status domain.WorktreeStatus) string {
	if parent := m.parents[status.Branch]; parent != "" {
		return parent
	}
	if status.IsParent {
		return domain.DashboardNoValue
	}
	return domain.DashboardUnknownParent
}

func stateText(status domain.WorktreeStatus) string {
	return worktreepicker.BuildStatus(status).Text
}

// baseText reports the divergence against the configured base branch, the
// referential `list` labels "base".
func baseText(status domain.WorktreeStatus) string {
	if status.CommitsAhead <= 0 {
		return domain.DashboardUpToDate
	}
	return fmt.Sprintf("%s%d", domain.BadgeGlyphAhead, status.CommitsAhead)
}

// originText reports the divergence against the branch's origin counterpart —
// the second referential, read from cached refs without ever fetching.
func originText(status domain.WorktreeStatus) string {
	badge, diverged := components.DivergenceBadge(components.DivergenceBadgeParams{
		State:  status.OriginState,
		Ahead:  status.OriginAhead,
		Behind: status.OriginBehind,
	})
	if diverged {
		return badge.Text
	}
	if status.OriginState == domain.DivergenceUnknown {
		return domain.DashboardNoValue
	}
	return domain.DashboardUpToDate
}

func (m Model) prText(branch string) string {
	if !m.prsLoaded {
		return domain.DashboardLoadingPRs
	}
	if banner := worktreepicker.GHBanner(m.ghConn); banner.Title != "" {
		return banner.Title
	}
	for _, pr := range m.prs {
		if pr.Branch == branch {
			return fmt.Sprintf("#%d %s (%s)", pr.Number, pr.Title, pr.State)
		}
	}
	return domain.DashboardNoPR
}

func createdText(status domain.WorktreeStatus) string {
	if status.CreatedAt.IsZero() {
		return domain.DashboardNoValue
	}
	return status.CreatedAt.Local().Format(domain.DashboardCreatedFormat)
}
