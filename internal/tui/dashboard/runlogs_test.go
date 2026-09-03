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

	model, cmd := model.openLogsTabOn("web")
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
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
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
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyEscape))

	if next.logsOpen() {
		t.Error("esc left the panel open")
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

func TestTheJobsLineNamesEveryDeclaredJobAndMarksWhatIsUp(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}, {Name: "pg"}}}
	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	line := stripANSI(model.logsJobsLine(60))

	for _, name := range []string{"web", "api", "pg"} {
		if !strings.Contains(line, name) {
			t.Errorf("line = %q, misses %q: a stopped job's tail is still readable", line, name)
		}
	}
	if !strings.Contains(line, domain.DetailJobUpGlyph) || !strings.Contains(line, domain.DetailJobDownGlyph) {
		t.Errorf("line = %q, want up and down told apart", line)
	}
	if !strings.HasSuffix(line, "http://web.wtm") {
		t.Errorf("line = %q, want the current job's address flush right", line)
	}
}

func TestSteppingTheJobStaysInsideItsWorktree(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.logsBranch, model.logsJob = "a", "web"

	model = model.stepLogsJob(1)
	if model.logsJob != "api" || model.logsBranch != "a" {
		t.Errorf("job = %q on %q, want api on a", model.logsJob, model.logsBranch)
	}

	model = model.stepLogsJob(1)
	if model.logsJob != "api" {
		t.Errorf("job = %q, want it clamped at the last one", model.logsJob)
	}

	model = model.stepLogsJob(-5)
	if model.logsJob != "web" {
		t.Errorf("job = %q, want it clamped at the first one", model.logsJob)
	}
}

func TestSteppingTheJobRetailsIt(t *testing.T) {
	asked := make(chan string, 2)
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) { asked <- req.Job; return nil, nil },
	}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	model = model.stepLogsJob(1)
	if cmd := model.tailLogsCmd(); cmd != nil {
		cmd()
	}

	select {
	case job := <-asked:
		if job != "api" {
			t.Errorf("tailed %q, want api", job)
		}
	default:
		t.Fatal("nothing was tailed after the job changed")
	}
}

// The logs view is hosted by the Services tab too, where the selected worktree
// and the tailed one part company.
func TestTheLogsAddressIsReadOffTheTailedBranch(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.addresses = map[string]map[string]domain.JobAddress{
		"a": {"web": {URL: "http://web.a.wtm"}},
		"b": {"web": {URL: "http://web.b.wtm"}},
	}
	model.logsBranch, model.logsJob = "b", "web"

	if got := model.logsAddress().URL; got != "http://web.b.wtm" {
		t.Errorf("address = %q, want the tailed worktree's own, not the selected one's", got)
	}
}

func TestTheLogsKeyOpensTheTabWithoutAPicker(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "worker"}, {Name: "web"}}}
	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.modal.open {
		t.Error("a modal opened, want the selection line to replace the picker")
	}
	if next.panelTab != panelLogs {
		t.Error("the panel stayed on DETAIL")
	}
	if next.logsJob != "web" {
		t.Errorf("logsJob = %q, want the first job that is up", next.logsJob)
	}
}

func TestTheLogsKeyDoesNothingWithoutARunModule(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.panelTab == panelLogs {
		t.Error("the LOGS tab opened with no job declared, want it inert")
	}
}

func TestArrowsWalkTheJobsWhileTheLogsTabIsUp(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyRight))

	if next.logsJob != "api" {
		t.Errorf("logsJob = %q, want the right arrow to walk the selection line", next.logsJob)
	}
	if next.cursor != 0 {
		t.Errorf("cursor = %d, want the list left alone while the logs tab has the keys", next.cursor)
	}
}

func TestEnterHandsRunviewTheJobOnScreen(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "api"

	request := model.watchLogsRequest()

	if request.Job != "api" {
		t.Errorf("Job = %q, want runview opened on the job on screen, not on the whole worktree", request.Job)
	}
	if request.Worktree != "a" {
		t.Errorf("Worktree = %q, want a", request.Worktree)
	}
}

func TestChangingTabClosesTheLogsView(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, key(keyTab))

	if next.panelTab != panelDetail {
		t.Error("the logs view survived the tab change and kept esc and enter, while not being drawn")
	}
}
