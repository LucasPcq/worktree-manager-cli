package dashboard

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

// detailMsg carries what worktree.Detail gathered for one branch, off the UI
// goroutine.
type detailMsg struct {
	branch string
	detail domain.WorktreeDetail
}

// detailTickMsg is the debounce firing for one branch. It only ever starts a
// load if that branch is still the selection: the point of the debounce is
// that a walk through many rows must not fire one git log per row crossed.
type detailTickMsg struct{ branch string }

// selectedBranch is the branch the current tab is pointing at, or "" when
// nothing is selected. It is what detailIsStale and every loader key their
// work on, rather than re-deriving it from the cursor each time.
func (m Model) selectedBranch() string {
	status, ok := m.selected()
	if !ok {
		return ""
	}
	return status.Branch
}

// triggerDetailReload starts (or restarts) the debounce for the current
// selection when it differs from before — a selection change is what §7 says
// schedules a load. detailLoading is set immediately so the UI already names
// what it is waiting on; detailSince is left at its zero value until the load
// actually starts (fireDetailTick), so the debounce wait is never counted
// against the grace delay — see detailIsStale.
func (m Model) triggerDetailReload(before string) (Model, tea.Cmd) {
	branch := m.selectedBranch()
	if branch == "" || branch == before {
		return m, nil
	}
	m.detailLoading, m.detailSince = branch, time.Time{}
	return m, m.scheduleDetail(branch)
}

// scheduleDetail debounces a detail load by domain.DashboardDetailDebounce. The
// tick only fires the actual load if branch is still the selection when it
// lands, so a fast walk through the list never launches one git log per row.
func (m Model) scheduleDetail(branch string) tea.Cmd {
	return tea.Tick(domain.DashboardDetailDebounce, func(time.Time) tea.Msg {
		return detailTickMsg{branch: branch}
	})
}

// fireDetailTick is the debounce landing. A tick for a branch that is no longer
// selected is dropped: the newer selection already scheduled its own tick, and
// this one starting a load now would be exactly the git-log-per-row-crossed
// this whole mechanism exists to prevent.
//
// This is where the load actually starts, so detailSince is set here: the
// grace delay in detailIsStale is spent entirely on the git call, never on the
// debounce wait ahead of it.
func (m Model) fireDetailTick(msg detailTickMsg) (Model, tea.Cmd) {
	if msg.branch != m.selectedBranch() {
		return m, nil
	}
	m.detailSince = time.Now()
	return m, tea.Batch(m.loadDetailCmd(msg.branch), m.spinner.Tick)
}

// reloadDetailCmd immediately (no debounce) relaunches a load for the current
// selection: what the poll and an explicit refresh do, as opposed to a
// selection change, which debounces. The cache is left untouched — old data
// stays on screen while this runs. detailSince is set here too, at the same
// point the load starts, so it means the same thing on both paths.
func (m Model) reloadDetailCmd() (Model, tea.Cmd) {
	branch := m.selectedBranch()
	if branch == "" {
		return m, nil
	}
	m.detailLoading, m.detailSince = branch, time.Now()
	return m, tea.Batch(m.loadDetailCmd(branch), m.spinner.Tick)
}

// invalidateDetail relaunches the load for branch when it is the one currently
// on screen, the way a finished operation refreshes what it just changed. A
// branch that is not selected is left alone: its stale cache entry is served
// again, at no cost, the next time it is selected.
func (m Model) invalidateDetail(branch string) (Model, tea.Cmd) {
	if branch == "" || branch != m.selectedBranch() {
		return m, nil
	}
	return m.reloadDetailCmd()
}

// applyDetail stores what a load returned and clears the loading marker, but
// only if it is still the load this branch is waiting on: an older,
// already-superseded load landing late must not clear a newer one's marker.
func (m Model) applyDetail(msg detailMsg) Model {
	m.details[msg.branch] = msg.detail
	if m.detailLoading == msg.branch {
		m.detailLoading = ""
	}
	return m
}

// loadDetailCmd runs worktree.Detail off the UI goroutine: it shells out to
// git, and calling it inside Update would freeze the whole program.
func (m Model) loadDetailCmd(branch string) tea.Cmd {
	params := m.detailParams(branch)
	return func() tea.Msg {
		return detailMsg{branch: branch, detail: worktree.Detail(params)}
	}
}

// detailParams gathers worktree.Detail's inputs from what the model already
// holds. Classification (blockers, PR matching) stays inside worktree.Detail;
// this only assembles what it needs.
func (m Model) detailParams(branch string) worktree.DetailParams {
	parent := m.parents[branch]
	return worktree.DetailParams{
		ProjectDir: m.params.ProjectDir,
		StateDir:   m.params.StateDir,
		Config:     m.params.Config,
		Status:     m.statusFor(branch),
		Parent:     parent,
		ParentPath: m.statusFor(parent).Path,
		Children:   m.childrenOf(branch),
		PRs:        m.prs,
		Commits:    domain.DashboardDetailCommits,
	}
}

// statusFor looks up a branch's WorktreeStatus. A branch with no local
// worktree (e.g. a parent that was never checked out) resolves to the zero
// value — an empty Path is a legitimate result, not an error.
//
// branch == "" is guarded explicitly: a detached worktree can carry an empty
// Branch through service/worktree.List, and without this guard a parentless
// branch (parent == "") would match that detached worktree's status and
// silently borrow its Path as ParentPath — wrong .env drift counters.
func (m Model) statusFor(branch string) domain.WorktreeStatus {
	if branch == "" {
		return domain.WorktreeStatus{}
	}
	for _, status := range m.statuses {
		if status.Branch == branch {
			return status
		}
	}
	return domain.WorktreeStatus{}
}

// childrenOf lists every branch whose recorded parent is branch, sorted: the
// selected branch's detail reloads regularly, and an unsorted map range would
// permute this line between refreshes, moving content §8 requires to hold
// still (state 2: "the content does not move a pixel").
func (m Model) childrenOf(branch string) []string {
	children := make([]string, 0, len(m.parents))
	for child, parent := range m.parents {
		if parent == branch {
			children = append(children, child)
		}
	}
	sort.Strings(children)
	return children
}

// detailIsStale reports whether the detail on screen is known to be behind: a
// load is in flight for the selected branch, it has actually started (a
// zero detailSince means still debouncing, not yet worth mentioning), and it
// has been running long enough (domain.DashboardSpinnerGrace) to be worth
// saying so. Below the grace delay, showing then hiding a marker would be
// flash dressed up as feedback.
func (m Model) detailIsStale() bool {
	return m.detailLoading != "" &&
		m.detailLoading == m.selectedBranch() &&
		!m.detailSince.IsZero() &&
		time.Since(m.detailSince) > domain.DashboardSpinnerGrace
}

// detailFreshnessMarker is the panel title's right-hand marker for freshness
// state 2 (spec §8): a spinner plus "refreshing" while stale data sits on
// screen, nothing at all otherwise — the normal, fresh state never signals.
func (m Model) detailFreshnessMarker() string {
	if !m.detailIsStale() {
		return ""
	}
	return m.spinner.View() + " " + styles.Muted.Render(domain.DashboardRefreshing)
}
