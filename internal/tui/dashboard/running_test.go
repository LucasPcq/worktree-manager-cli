package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func runningModel(t *testing.T, running ...string) Model {
	t.Helper()
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a", "idle", "b"), parents: map[string]string{}})
	model.tab = tabRunning
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}}

	now := time.Now()
	addresses := map[string]map[string]domain.JobAddress{}
	for _, branch := range running {
		model.jobs = append(model.jobs, domain.JobInfo{
			Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/" + branch, StartedAt: now.Add(-time.Hour),
		})
		addresses[branch] = map[string]domain.JobAddress{"web": {URL: "http://web." + branch + ".wtm"}}
	}
	model.addresses = addresses
	return model.withBoard()
}

func TestRunningTabListsOneBlockPerWorktreeThatRuns(t *testing.T) {
	model := runningModel(t, "a")

	body := stripANSI(strings.Join(model.runningBody(model.layout()), "\n"))

	if !strings.Contains(body, "a") {
		t.Errorf("body = %q, want the running worktree named", body)
	}
	if !strings.Contains(body, "http://web.a.wtm") {
		t.Errorf("body = %q, want the url visible — that is what this tab is for", body)
	}
	if strings.Contains(body, "idle") {
		t.Errorf("body = %q, want an idle worktree left out", body)
	}
	if strings.Contains(body, "worker") {
		t.Errorf("body = %q, want a stopped job left out: this tab answers what runs", body)
	}
}

func TestRunningTabSaysWhatToDoWhenNothingRuns(t *testing.T) {
	model := runningModel(t)

	body := stripANSI(strings.Join(model.runningBody(model.layout()), "\n"))

	if !strings.Contains(body, domain.DashboardRunningEmpty) {
		t.Errorf("body = %q, want it to say nothing runs", body)
	}
	if !strings.Contains(body, domain.DashboardRunningEmptyHint) {
		t.Errorf("body = %q, want the way to start something named", body)
	}
}

func TestRunningTabTakesTheWholeWidth(t *testing.T) {
	model := runningModel(t, "a")

	layout := model.layout()

	if layout.DetailVisible {
		t.Error("the detail shows beside the Running tab, want the tab alone")
	}
	if layout.List.Width != testWidth {
		t.Errorf("List.Width = %d, want the full %d", layout.List.Width, testWidth)
	}
}

func TestRunningTabCursorSelectsTheWorktreeOfItsBlock(t *testing.T) {
	model := runningModel(t, "a", "b")

	model.runningCursor = 1
	selected, ok := model.selectedRunning()

	if !ok || selected.Branch != "b" {
		t.Errorf("selected = %+v, want b: the cursor navigates worktrees here", selected)
	}
}

func TestRunningTabArrowsWalkTheBlocks(t *testing.T) {
	model := runningModel(t, "a", "b")

	model = model.moveCursor(1)
	if model.runningCursor != 1 {
		t.Errorf("runningCursor = %d, want the arrows to walk the blocks", model.runningCursor)
	}

	model = model.moveCursor(5)
	if model.runningCursor != 1 {
		t.Errorf("runningCursor = %d, want it clamped to the last block", model.runningCursor)
	}
}

func TestRunningTabMenuOffersRunOnly(t *testing.T) {
	model := runningModel(t, "a")
	model.menuKind = menuForWorktree

	items := model.menuItems()

	if len(items) == 0 {
		t.Fatal("no entries at all")
	}
	for _, item := range items {
		if item.label == domain.DashboardMenuSectionGit {
			t.Error("the Running tab offers GIT, want run actions only: this tab does not speak of git")
		}
		if item.danger {
			t.Errorf("entry %q is destructive, want none here", item.label)
		}
	}
	if items[0].kind == menuEntrySeparator {
		t.Error("the block opens with a rule, want nothing above the first heading to separate")
	}
}

func TestRunningTabActionsTargetTheBlockUnderTheCursor(t *testing.T) {
	model := runningModel(t, "a", "b")
	model.runningCursor = 1

	target, ok := model.selected()

	if !ok || target.Branch != "b" {
		t.Errorf("selected = %+v, want b: the Running tab's cursor is what actions act on", target)
	}
	if target.Path != "/tmp/b" {
		t.Errorf("Path = %q, want the worktree's own, not the block's copy", target.Path)
	}
}

func TestRunningTabCountsWhatIsUp(t *testing.T) {
	model := runningModel(t, "a", "b")

	if got := model.countText(); !strings.Contains(got, "2") {
		t.Errorf("countText = %q, want the two running jobs counted", got)
	}
}

// The addresses are what this tab is read down: a name column restarting at
// each worktree would break that read.
func TestRunningTabAlignsAddressesAcrossBlocks(t *testing.T) {
	model := runningModel(t, "a", "b")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "database"}}}
	model.jobs = append(model.jobs, domain.JobInfo{
		Name: "database", Status: domain.JobStatusRunning, WorkDir: "/tmp/b", StartedAt: time.Now(),
	})
	model.addresses["b"]["database"] = domain.JobAddress{Ports: []int{5432}}
	model = model.withBoard()

	lines := model.runningBody(model.layout())
	columns := make([]int, 0, 3)
	for _, line := range lines {
		plain := stripANSI(line)
		if index := strings.Index(plain, "http://"); index >= 0 {
			columns = append(columns, index)
			continue
		}
		if index := strings.Index(plain, ":5432"); index >= 0 {
			columns = append(columns, index)
		}
	}

	if len(columns) < 3 {
		t.Fatalf("found %d address cells, want one per running job", len(columns))
	}
	for _, column := range columns[1:] {
		if column != columns[0] {
			t.Errorf("addresses start at %v, want one column across every block", columns)
			break
		}
	}
}

// The board was rebuilt by countText, runningBody, selected, rowCount, clickRow
// and selectedRunning — six times a frame, each with its own time.Now().
func TestTheBoardIsBuiltByThePollNotByTheRenderer(t *testing.T) {
	model := runningModel(t, "a")

	before := model.board
	_ = model.runningBody(model.layout())
	_, _ = model.selected()
	_ = model.countText()

	if len(before) == 0 {
		t.Fatal("board is empty, want it built when the jobs landed")
	}
	if &model.board[0] != &before[0] {
		t.Error("the renderer rebuilt the board, want it read off the model")
	}
}
