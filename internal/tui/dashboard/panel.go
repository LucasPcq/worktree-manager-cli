package dashboard

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	panelDetail = iota
	panelLogs
)

// logsAvailable is what makes the LOGS tab reachable: a project that declares
// jobs. Not one that is running them — a job's tail is read back after it
// stopped, which is when it matters most.
func (m Model) logsAvailable() bool { return len(m.runConfig.Jobs) > 0 }

// panelTabsLine heads the right-hand panel, in the place its title used to
// take. The freshness marker keeps its flush-right place: it dates the panel,
// not the tab.
func (m Model) panelTabsLine(width int) string {
	tabs := m.marks().Mark(zonePanelTabDtl, m.panelTabCell(panelDetail, domain.DashboardPanelTabDetail)) +
		styles.DashboardPanelTabIdle.Render(domain.DashboardPanelTabSep) +
		m.marks().Mark(zonePanelTabLogs, m.panelTabCell(panelLogs, domain.DashboardPanelTabLogs))

	return spread(tabs, m.detailFreshnessMarker(), width)
}

func (m Model) panelTabCell(tab int, label string) string {
	if tab == panelLogs && !m.logsAvailable() {
		return styles.DashboardPanelTabOff.Render(label)
	}
	if tab == m.panelTab {
		return styles.DashboardPanelTabActive.Render(label)
	}
	return styles.DashboardPanelTabIdle.Render(label)
}
