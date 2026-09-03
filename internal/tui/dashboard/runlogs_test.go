package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func logsModel(t *testing.T, params RunParams, branches ...string) Model {
	t.Helper()
	model := New(params)
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, worktreesMsg{statuses: statuses(branches...), parents: map[string]string{}})
}

func TestOpeningTheLogsPanelTailsTheJob(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) {
			if req.Job != "web" || req.WorkDir != "/tmp/a" {
				t.Errorf("request = %+v, want web in /tmp/a", req)
			}
			if req.Lines != domain.DashboardLogsLines {
				t.Errorf("lines = %d, want %d", req.Lines, domain.DashboardLogsLines)
			}
			return []string{"ready in 380ms"}, nil
		},
	}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	model, cmd := model.openLogsPanel("web")
	if model.logsJob != "web" || model.logsBranch != "a" {
		t.Fatalf("panel = %q/%q, want web on a", model.logsBranch, model.logsJob)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the tail read off the UI goroutine")
	}

	msg, ok := cmd().(logsTailMsg)
	if !ok {
		t.Fatalf("msg = %T, want logsTailMsg", cmd())
	}
	model = model.applyLogsTail(msg)
	if len(model.logsLines) != 1 || model.logsLines[0] != "ready in 380ms" {
		t.Errorf("lines = %v, want the tail", model.logsLines)
	}
}

func TestASupersededTailNeverLandsOnTheOpenPanel(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"kept"}

	model = model.applyLogsTail(logsTailMsg{branch: "a", job: "api", lines: []string{"other job"}})

	if len(model.logsLines) != 1 || model.logsLines[0] != "kept" {
		t.Errorf("lines = %v, want the open panel's own tail untouched", model.logsLines)
	}
}

func TestSelectingAnotherWorktreeClosesTheLogsPanel(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model.logsBranch, model.logsJob = "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyDown))

	if next.logsOpen() {
		t.Error("the panel survived the move, want it closed: it speaks of one worktree's job")
	}
}

func TestLogsPanelHeadsWithTheJobAndKeepsTheLastLines(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.jobs = []domain.JobInfo{{
		Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a",
		StartedAt: time.Now().Add(-72 * time.Minute),
	}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"line 1", "line 2", "line 3"}

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "web") {
		t.Errorf("body = %q, want the job named", body)
	}
	if !strings.Contains(body, "http://web.wtm") {
		t.Errorf("body = %q, want the address on the header", body)
	}
	if !strings.Contains(body, "line 3") {
		t.Errorf("body = %q, want the newest line kept", body)
	}
	if !strings.Contains(body, domain.DashboardLogsHint) {
		t.Errorf("body = %q, want the way out named", body)
	}
}

func TestLogsPanelDropsTheOldestLinesWhenTheBudgetIsShort(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.logsBranch, model.logsJob = "a", "web"
	for index := range 200 {
		model.logsLines = append(model.logsLines, "line "+string(rune('a'+index%26))+string(rune('0'+index/26)))
	}
	model.logsLines[len(model.logsLines)-1] = "newest"
	model.logsLines[0] = "oldest"

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "newest") {
		t.Error("the newest line was dropped, want the oldest ones to go first")
	}
	if strings.Contains(body, "oldest") {
		t.Error("the oldest line survived a short budget, want it dropped")
	}
}

func TestLogsPanelSaysWhyItCouldNotRead(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.logsBranch, model.logsJob = "a", "web"
	model.logsErr = errors.New("no log for web")

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "no log for web") {
		t.Errorf("body = %q, want the failure named rather than an empty panel", body)
	}
}

func TestTheDetailPanelYieldsToTheLogsPanelWhileItIsOpen(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"tailed"}

	body := stripANSI(strings.Join(model.detailBody(model.layout()), "\n"))

	if strings.Contains(body, domain.DetailSectionLinks) {
		t.Errorf("body = %q, want the sections replaced by the tail", body)
	}
	if !strings.Contains(body, "tailed") {
		t.Errorf("body = %q, want the tail shown", body)
	}
}

func TestEscapeLeavesTheLogsPanelForTheDetail(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.logsBranch, model.logsJob = "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyEscape))

	if next.logsOpen() {
		t.Error("esc left the panel open")
	}
}

func TestTheLogsKeyOpensTheJobPicker(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if !next.modal.open {
		t.Fatal("the picker never opened: the detail has no cursor to name a job with")
	}
	if next.logsOpen() {
		t.Error("a job was opened without being chosen")
	}
}

func TestTheLogsKeyRefusesAProjectWithNoRunModule(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.modal.open {
		t.Error("a picker opened with nothing in it, want the refusal")
	}
	if !strings.Contains(strings.Join(next.outputLines, "\n"), domain.DashboardRunNotConfigured) {
		t.Errorf("output = %v, want it to name `wtm run init`", next.outputLines)
	}
}

func TestADismissedPickerOpensNothing(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	next := update(model, logsJobMsg{})

	if next.logsOpen() {
		t.Error("a dismissed picker opened a panel")
	}
}

func TestClickingAJobWithNoURLOpensItsLogs(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(logsRequest) ([]string, error) { return []string{"ready"}, nil },
	}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "pg"}}}
	model.jobs = []domain.JobInfo{{Name: "pg", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"pg": {Ports: []int{5432}}}}
	renderAndWait(t, model, runRowZone("pg"))

	zone := model.zones.Get(runRowZone("pg"))
	next, _ := updateCmd(model, click(zone.StartX+3, zone.StartY))

	if next.logsJob != "pg" {
		t.Errorf("logsJob = %q, want the click to lead to what the job does have", next.logsJob)
	}
}
