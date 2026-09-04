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
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
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
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
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

// The tab always opens: a greyed-out tab left the reader guessing, and a
// project with no run module has something to be told rather than a door that
// does not answer.
func TestTheLogsTabOpensWithoutARunModuleAndSaysWhatIsMissing(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.panelTab != panelLogs {
		t.Fatal("the LOGS tab did not open")
	}
	body := stripANSI(strings.Join(next.detailBody(next.layout()), "\n"))
	if !strings.Contains(body, domain.DashboardLogsNoModule) {
		t.Errorf("body = %q, want it to say the project runs nothing", body)
	}
	if !strings.Contains(body, domain.DashboardLogsNoModuleHint) {
		t.Errorf("body = %q, want `wtm run init` named", body)
	}
}

func TestTheLogsViewTellsANeverRunJobFromAQuietOne(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	down := stripANSI(strings.Join(model.logsViewBody(logsViewParams{Width: 60, Height: 20}), "\n"))
	if !strings.Contains(down, domain.DashboardLogsNeverRan) {
		t.Errorf("body = %q, want a stopped job told it has never run here", down)
	}

	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	up := stripANSI(strings.Join(model.logsViewBody(logsViewParams{Width: 60, Height: 20}), "\n"))
	if !strings.Contains(up, domain.DashboardLogsQuiet) {
		t.Errorf("body = %q, want a running job that wrote nothing told apart", up)
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
	if len(request.Worktrees) != 1 || request.Worktrees[0] != "a" {
		t.Errorf("Worktrees = %v, want a", request.Worktrees)
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

// The tail is shared by the two hosts. Closing one of them used to drop it, so
// a Services row of another worktree opened its logs and lost them at once —
// triggerDetailReload closes the panel on every change of selected branch.
func TestClosingOneHostLeavesTheOtherOneItsTail(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.servicesLogs = true
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"kept"}

	model = model.closePanelLogs()

	if model.logsJob != "web" || len(model.logsLines) != 1 {
		t.Errorf("logs = %q/%v, want the Services view's tail untouched", model.logsJob, model.logsLines)
	}

	model = model.closeServiceLogs()
	if model.logsJob != "" || model.logsLines != nil {
		t.Errorf("logs = %q/%v, want them dropped once no host shows them", model.logsJob, model.logsLines)
	}
}

// The chips were marked but nothing ever looked them up: a zone with no
// handler is a click that lands nowhere.
func TestClickingAJobChipSwitchesToIt(t *testing.T) {
	tailed := make(chan string, 2)
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) { tailed <- req.Job; return nil, nil },
	}, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	renderAndWait(t, model, logsJobZone("api"))

	zone := model.zones.Get(logsJobZone("api"))
	next, cmd := updateCmd(model, click(zone.StartX, zone.StartY))

	if next.logsJob != "api" {
		t.Fatalf("logsJob = %q, want the chip that was clicked", next.logsJob)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the new job tailed")
	}
	cmd()
	select {
	case job := <-tailed:
		if job != "api" {
			t.Errorf("tailed %q, want api", job)
		}
	default:
		t.Fatal("nothing was tailed")
	}
}

func TestClickingTheChipAlreadyShownChangesNothing(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	model.logsLines = []string{"kept"}
	renderAndWait(t, model, logsJobZone("web"))

	zone := model.zones.Get(logsJobZone("web"))
	next, _ := updateCmd(model, click(zone.StartX, zone.StartY))

	if len(next.logsLines) != 1 {
		t.Errorf("lines = %v, want the tail on screen left alone", next.logsLines)
	}
}

func TestClickingTheAddressInTheLogsViewOpensIt(t *testing.T) {
	opened := make(chan string, 1)
	model := logsModel(t, RunParams{
		URLOpener: func(url string) error { opened <- url; return nil },
	}, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	renderAndWait(t, model, logsURLZone())

	zone := model.zones.Get(logsURLZone())
	_, cmd := updateCmd(model, click(zone.StartX, zone.StartY))
	if cmd == nil {
		t.Fatal("clicking the address in the logs view did nothing")
	}
	cmd()

	select {
	case got := <-opened:
		if got != "http://web.wtm" {
			t.Errorf("opened %q, want http://web.wtm", got)
		}
	default:
		t.Fatal("nothing opened")
	}
}

// A job that publishes no address has none to mark: a zone over empty text
// would swallow clicks meant for the chips beside it.
func TestAJobWithoutAnAddressMarksNoZoneInTheLogsView(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "pg"}}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"pg": {Ports: []int{5432}}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "pg"
	renderAndWait(t, model, zoneDetail)

	if !model.zones.Get(logsURLZone()).IsZero() {
		t.Error("an address zone was marked for a job that publishes none")
	}
}
