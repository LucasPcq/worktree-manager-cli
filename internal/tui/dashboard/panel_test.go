package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestThePanelHeadsWithItsTwoTabs(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	line := stripANSI(model.panelTabsLine(60))

	if !strings.Contains(line, domain.DashboardPanelTabDetail) || !strings.Contains(line, domain.DashboardPanelTabLogs) {
		t.Errorf("line = %q, want both tabs named", line)
	}
	if lipgloss.Width(line) != 60 {
		t.Errorf("width = %d, want the line to fill the panel", lipgloss.Width(line))
	}
}

// The logs tab reads what a job persisted, running or not: a job that just
// crashed is the one whose tail matters most. What makes it inert is a project
// with no run module at all.
func TestTheLogsTabIsInertOnlyWithoutARunModule(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	if model.logsAvailable() {
		t.Error("logsAvailable = true with no job declared, want the tab inert")
	}

	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	if !model.logsAvailable() {
		t.Error("logsAvailable = false with a declared job that is down, want it reachable")
	}
}

func TestTheFreshnessMarkerKeepsItsPlaceOnTheTabLine(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailLoading, model.detailSince = "a", time.Now().Add(-time.Hour)

	line := stripANSI(model.panelTabsLine(60))

	if !strings.Contains(line, domain.DashboardRefreshing) {
		t.Errorf("line = %q, want the marker kept: it dates the panel, not the tab", line)
	}
}
