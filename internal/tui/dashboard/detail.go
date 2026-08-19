package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const detailLabelWidth = 9

func (m Model) renderDetail(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:       layout.Detail,
		Title:      domain.DashboardDetailTitle,
		TitleRight: m.detailFreshnessMarker(),
		Body:       m.detailBody(layout),
		Zone:       zoneDetail,
	})
}

// detailSection is one group of fields under its own heading. The panel reads as
// three questions — where it is, where it stands, what is open on it — rather
// than as one list of nine labels.
type detailSection struct {
	title  string
	fields [][2]string
}

func (m Model) detailBody(layout domain.DashboardLayout) []string {
	width := layout.Detail.Width - borderWidth - paddingWidth
	status, ok := m.selected()
	if !ok {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptySelection, width))}
	}

	lines := []string{
		spread(styles.DashboardBranch.Render(truncate(status.Branch, width)),
			worktreepicker.BuildStatus(status).Render(), width),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, max(width, 0))),
	}
	for _, section := range m.detailSections(status) {
		lines = append(lines, "", styles.DashboardSectionTitle.Render(truncate(section.title, width)), "")
		for _, field := range section.fields {
			lines = append(lines, styles.DashboardLabel.Render(pad(field[0], detailLabelWidth))+
				styles.DashboardValue.Render(truncate(field[1], max(width-detailLabelWidth, 0))))
		}
	}
	return lines
}

func (m Model) detailSections(status domain.WorktreeStatus) []detailSection {
	// The working-tree state is the pill in the heading, not a field of its own.
	state := [][2]string{
		{domain.DashboardLabelBase, baseText(status)},
		{domain.DashboardLabelOrigin, originText(status)},
	}
	if status.RebaseInProgress {
		state = append(state, [2]string{domain.DashboardLabelRebase, domain.DashboardRebaseInProgress})
	}

	return []detailSection{
		{
			title: domain.DashboardSectionWorktree,
			fields: [][2]string{
				{domain.DashboardLabelPath, status.Path},
				{domain.DashboardLabelParent, m.parentOf(status)},
				{domain.DashboardLabelCreated, createdText(status)},
			},
		},
		{title: domain.DashboardSectionDivergence, fields: state},
		{
			title:  domain.DashboardSectionReview,
			fields: [][2]string{{domain.DashboardLabelPR, m.prText(status.Branch)}},
		},
	}
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
			return fmt.Sprintf(domain.DashboardPRFmt, pr.Number, pr.Title, pr.State)
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
