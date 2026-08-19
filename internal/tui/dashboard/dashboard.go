// Package dashboard renders `wtm ui`: a full-screen view of the repository's
// worktrees, and the surface the create and clean flows are run from. It draws
// and it asks; what a run does is internal/flow's business, never its own.
package dashboard

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// RunParams holds what the dashboard needs to keep itself fed. The PR loader is
// injected so the slow `gh` call stays a surface concern, as in the pickers.
type RunParams struct {
	ProjectDir string
	StateDir   string
	// Cwd is the directory the shell was in when it launched `wtm ui` — not
	// necessarily ProjectDir, which LoadConfig may have resolved upward. It is
	// what the active-worktree match is run against.
	Cwd      string
	Config   domain.Config
	PRLoader worktreepicker.PRLoaderFunc
}

// OutputLineMsg appends one line to the bottom output panel. Every phase of a
// running flow — hook output included — reaches the panel through it.
type OutputLineMsg struct{ Text string }

type worktreesMsg struct {
	statuses  []domain.WorktreeStatus
	parents   map[string]string
	fetchedAt time.Time
	err       error
}

type prsMsg struct {
	prs  []domain.PRInfo
	conn domain.GHConnection
}

type pollMsg struct{}

type treeMsg struct {
	rows []domain.TreeRow
	err  error
}

var tabs = []string{domain.DashboardTabWorktrees, domain.DashboardTabTree}

// tabTree is the index of the Tree tab in tabs; the renderer and the loader both
// key off it rather than off its title.
const (
	tabWorktrees = iota
	tabTree
)

// Model is the dashboard's root Bubbletea model. It owns its own zone manager
// so hit-testing is per-program state rather than a package global.
type Model struct {
	params     RunParams
	listParams domain.ListParams
	zones      *zone.Manager

	width  int
	height int

	tab      int
	statuses []domain.WorktreeStatus
	parents  map[string]string
	cursor   int
	offset   int
	loaded   bool
	loadErr  error
	loading  bool
	// activeBranch is the worktree the cwd is under, deduced from a path-prefix
	// match — no git call. Empty until a surface computes and sets it.
	activeBranch string
	// repoName and baseBranch name the header's context line; both are fixed for
	// the life of the program. fetchedAt dates the last successful fetch (zero
	// when the repository has never fetched) and is refreshed on every reload.
	repoName   string
	baseBranch string
	fetchedAt  time.Time

	prs       []domain.PRInfo
	ghConn    domain.GHConnection
	prsLoaded bool

	outputLines    []string
	outputOffset   int
	outputExpanded bool

	treeRows    []domain.TreeRow
	treeCursor  int
	treeOffset  int
	treeLoaded  bool
	treeLoading bool
	treeErr     error

	detailOpen bool
	showHelp   bool

	// details caches the last detail loaded per branch, invalidated (never
	// emptied) by the poll and by a finished operation, so the panel keeps
	// showing "true a second ago" through a reload instead of going blank.
	details       map[string]domain.WorktreeDetail
	detailLoading string
	detailSince   time.Time
	spinner       spinner.Model

	menuOpen   bool
	menuKind   menuKind
	menuCursor int
	// menuAnchor is the cell the context menu hangs from: the click, or the
	// selected row when the keyboard opened it.
	menuAnchor domain.Rect

	// msgs carries what the flow goroutines post; listenCmd is its only reader.
	msgs  chan tea.Msg
	ops   operations
	modal modal
	// selectBranch is the worktree a finished run wants selected once the list
	// catches up with it.
	selectBranch string
}

// New builds the dashboard model. Callers outside a program must Close the
// returned model's zone manager; Run does it for them.
func New(params RunParams) Model {
	return Model{
		params: params,
		listParams: domain.ListParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Config:     params.Config,
		},
		zones:      zone.New(),
		msgs:       make(chan tea.Msg, domain.DashboardMsgBuffer),
		ghConn:     domain.GHConnectionOK,
		loading:    true,
		details:    map[string]domain.WorktreeDetail{},
		spinner:    components.MutedSpinner(),
		repoName:   filepath.Base(params.ProjectDir),
		baseBranch: params.Config.Project.Worktrees.BaseBranch,
	}
}

func (m Model) Close() { m.zones.Close() }

// Run opens the dashboard on the alternate screen. It restores the terminal on
// exit without re-emitting anything into the scrollback.
func Run(params RunParams) error {
	model := New(params)
	defer model.Close()

	if _, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	// The spinner is started on demand, at the point a detail load actually
	// begins (fireDetailTick, reloadDetailCmd) — not here, or it would tick for
	// the life of the program whether or not anything is loading.
	return tea.Batch(m.loadWorktreesCmd(false), m.loadPRsCmd(), pollCmd(), listenCmd(m.msgs))
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.DashboardPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

// loadWorktreesCmd re-lists the worktrees off the UI thread. fetch reaches the
// remote (the `r` key); the poll never does, so the origin badges stay a
// deliberate, user-triggered refresh.
func (m Model) loadWorktreesCmd(fetch bool) tea.Cmd {
	listParams, stateDir, projectDir := m.listParams, m.params.StateDir, m.params.ProjectDir
	return func() tea.Msg {
		list := worktree.List
		if fetch {
			list = worktree.Refresh
		}
		statuses, err := list(listParams)
		if err != nil {
			return worktreesMsg{err: err}
		}
		parents := make(map[string]string, len(statuses))
		for _, status := range statuses {
			parents[status.Branch] = worktree.ParentBranch(worktree.ParentBranchParams{
				StateDir: stateDir,
				Branch:   status.Branch,
			})
		}
		fetchedAt := worktree.LastFetchAt(worktree.LastFetchAtParams{ProjectDir: projectDir})
		return worktreesMsg{statuses: statuses, parents: parents, fetchedAt: fetchedAt}
	}
}

// loadTreeCmd builds the forest off the UI thread. It costs a rev-list per node,
// which is why it is only ever asked for once the Tree tab has been opened.
func (m Model) loadTreeCmd() tea.Cmd {
	listParams := m.listParams
	return func() tea.Msg {
		forest, err := worktree.BuildTree(worktree.BuildTreeParams{
			ProjectDir: listParams.ProjectDir,
			StateDir:   listParams.StateDir,
			Config:     listParams.Config,
		})
		if err != nil {
			return treeMsg{err: err}
		}
		return treeMsg{rows: rules.FlattenForest(forest)}
	}
}

// treeCmd asks for a rebuild only when the tab has been opened at least once: a
// user who never looks at the tree never pays for it.
func (m Model) treeCmd() tea.Cmd {
	if !m.treeLoaded && m.tab != tabTree {
		return nil
	}
	return m.loadTreeCmd()
}

func (m Model) loadPRsCmd() tea.Cmd {
	loader := m.params.PRLoader
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		prs, conn := loader()
		return prsMsg{prs: prs, conn: conn}
	}
}

func (m Model) layout() domain.DashboardLayout {
	return rules.ComputeDashboardLayout(rules.DashboardLayoutParams{
		Width:          m.width,
		Height:         m.height,
		OutputExpanded: m.outputExpanded,
		DetailOpen:     m.detailOpen,
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.modal = m.modal.resize(msg.Width, msg.Height)
		return m.reflow(), nil

	case tea.KeyMsg:
		before := m.selectedBranch()
		model, cmd := m.handleKey(msg)
		return withDetailTrigger(before, model, cmd)

	case tea.MouseMsg:
		before := m.selectedBranch()
		model, cmd := m.handleMouse(msg)
		return withDetailTrigger(before, model, cmd)

	case worktreesMsg:
		before := m.selectedBranch()
		next := m.applyWorktrees(msg)
		return next.triggerDetailReload(before)

	case prsMsg:
		m.prs, m.ghConn, m.prsLoaded = msg.prs, msg.conn, true
		return m, nil

	case pollMsg:
		if m.loading {
			return m, pollCmd()
		}
		m.loading = true
		// The tree only refreshes on the poll while it is on screen; rebuilding a
		// panel nobody is looking at costs a rev-list per node for nothing.
		tree := tea.Cmd(nil)
		if m.tab == tabTree {
			tree = m.loadTreeCmd()
		}
		// Same reasoning for the detail: five subprocesses every poll to keep a
		// panel nobody is looking at "fresh" is exactly the jamais-dans-le-poll
		// rule (§7) would forbid if the panel were on screen — it is spent only
		// when it is. The poll never clears m.details either way; when it does
		// reload, old data stays on screen underneath it.
		detailCmd := tea.Cmd(nil)
		if m.layout().DetailVisible {
			m, detailCmd = m.reloadDetailCmd()
		}
		return m, tea.Batch(m.loadWorktreesCmd(false), tree, detailCmd, pollCmd())

	case treeMsg:
		before := m.selectedBranch()
		next := m.applyTree(msg)
		return next.triggerDetailReload(before)

	case OutputLineMsg:
		return m.appendOutput(msg), nil

	case flowMsg:
		model, cmd := m.applyFlow(msg.inner)
		return model, tea.Batch(cmd, listenCmd(m.msgs))

	case opDoneMsg:
		return m.finishOp(msg)

	case detailMsg:
		return m.applyDetail(msg), nil

	case detailTickMsg:
		return m.fireDetailTick(msg)

	case spinner.TickMsg:
		// Re-arming unconditionally would tick at 12fps for the life of the
		// program, idle included: the loop only continues while a detail load is
		// actually in flight, and dies on its own the moment applyDetail clears
		// detailLoading.
		if m.detailLoading == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case modalLoadedMsg, formReadyMsg:
		return m.updateModal(msg)
	}

	return m, nil
}

// withDetailTrigger folds a detail-reload check onto whatever a key or mouse
// event already produced. handleKey and handleMouse return tea.Model because
// they also field the overlays (modal, menu, help), which are not this Model;
// the comma-ok keeps a foreign result harmless instead of panicking.
func withDetailTrigger(before string, next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	model, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	model, detailCmd := model.triggerDetailReload(before)
	return model, tea.Batch(cmd, detailCmd)
}

func (m Model) applyTree(msg treeMsg) Model {
	m.treeLoading, m.treeErr = false, msg.err
	if msg.err != nil {
		return m
	}
	m.treeRows, m.treeLoaded = msg.rows, true
	m.treeCursor = rules.ClampIndex(m.treeCursor, len(m.treeRows))
	return m.reflow()
}

// applyWorktrees keeps the selection on a valid row: a worktree removed under
// the poll must not leave the cursor pointing past the end.
func (m Model) applyWorktrees(msg worktreesMsg) Model {
	m.loading = false
	m.loadErr = msg.err
	if msg.err != nil {
		return m
	}
	m.statuses, m.parents, m.loaded = msg.statuses, msg.parents, true
	m.fetchedAt = msg.fetchedAt
	m.activeBranch = rules.ActiveWorktree(rules.ActiveWorktreeParams{
		Cwd:      m.params.Cwd,
		Statuses: m.statuses,
	})
	m.cursor = rules.ClampIndex(m.cursor, len(m.statuses))
	return m.selectRequested().reflow()
}

// selectRequested lands the cursor on the worktree a finished run created, the
// one time the list comes back holding it.
func (m Model) selectRequested() Model {
	if m.selectBranch == "" {
		return m
	}
	for index, status := range m.statuses {
		if status.Branch == m.selectBranch {
			m.cursor, m.selectBranch = index, ""
			return m
		}
	}
	return m
}

func (m Model) updateModal(msg tea.Msg) (Model, tea.Cmd) {
	modal, cmd := m.modal.update(msg)
	m.modal = modal
	return m, cmd
}

func (m Model) reflow() Model {
	layout := m.layout()
	m.offset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.cursor,
		Total:   len(m.statuses),
		Visible: layout.ListRows,
		Offset:  m.offset,
	})
	m.treeOffset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.treeCursor,
		Total:   len(m.treeRows),
		Visible: layout.TreeRows,
		Offset:  m.treeOffset,
	})
	m.outputOffset = rules.DashboardClampOffset(rules.DashboardOffsetParams{
		Offset:  m.outputOffset,
		Total:   len(m.outputLines),
		Visible: layout.OutputLines,
	})
	return m
}

// selected is the worktree the current tab is pointing at. On the Tree tab a row
// is a node, and a virtual node stands for a branch with no worktree — so it
// selects nothing, and everything keyed off the selection (the detail panel, the
// context menu) correctly finds there is nothing to act on.
func (m Model) selected() (domain.WorktreeStatus, bool) {
	if m.tab == tabTree {
		return m.selectedTreeWorktree()
	}
	if m.cursor < 0 || m.cursor >= len(m.statuses) {
		return domain.WorktreeStatus{}, false
	}
	return m.statuses[m.cursor], true
}

func (m Model) selectedTreeNode() (domain.TreeNode, bool) {
	if m.treeCursor < 0 || m.treeCursor >= len(m.treeRows) {
		return domain.TreeNode{}, false
	}
	return m.treeRows[m.treeCursor].Node, true
}

func (m Model) selectedTreeWorktree() (domain.WorktreeStatus, bool) {
	node, ok := m.selectedTreeNode()
	if !ok || node.IsVirtual {
		return domain.WorktreeStatus{}, false
	}
	for _, status := range m.statuses {
		if status.Branch == node.Branch {
			return status, true
		}
	}
	return domain.WorktreeStatus{}, false
}

func (m Model) moveCursor(delta int) Model {
	if m.tab == tabTree {
		m.treeCursor = rules.ClampIndex(m.treeCursor+delta, len(m.treeRows))
		return m.reflow()
	}
	m.cursor = rules.ClampIndex(m.cursor+delta, len(m.statuses))
	return m.reflow()
}

// rowCount is how many rows the current tab holds, for the keys that jump by a
// page or to an end.
func (m Model) rowCount() int {
	if m.tab == tabTree {
		return len(m.treeRows)
	}
	return len(m.statuses)
}

func (m Model) scrollOutput(delta int) Model {
	m.outputOffset = max(m.outputOffset+delta, 0)
	return m.reflow()
}

func (m Model) refresh() (Model, tea.Cmd) {
	m.loading, m.prsLoaded = true, false
	m.treeLoading = m.treeLoaded || m.tab == tabTree
	next, detailCmd := m.reloadDetailCmd()
	return next, tea.Batch(next.loadWorktreesCmd(true), next.loadPRsCmd(), next.treeCmd(), detailCmd)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// A modal owns the keyboard while it is up: it is a question, and nothing
	// behind it may be acted on before it is answered.
	if m.modal.open {
		if key == keyInterrupt {
			return m, tea.Quit
		}
		return m.updateModal(msg)
	}

	// The overlay documents "q · ctrl+c quit", so it must not swallow them.
	if m.showHelp {
		if key == keyQuit || key == keyInterrupt {
			return m, tea.Quit
		}
		if key == keyHelp || key == keyEscape {
			m.showHelp = false
		}
		return m, nil
	}

	// An open menu takes the keys it uses and lets every other one through, after
	// closing itself: a dropdown must never trap the keyboard.
	if m.menuOpen {
		switch key {
		case keyUp, keyVimUp:
			return m.moveMenu(-1), nil
		case keyDown, keyVimDown:
			return m.moveMenu(1), nil
		case keyEnter:
			return m.activateMenu(m.menuCursor)
		case keyEscape, keyMenu:
			return m.closeMenu(), nil
		}
		m = m.closeMenu()
	}

	layout := m.layout()

	switch key {
	case keyInterrupt, keyQuit:
		return m, tea.Quit
	case keyHelp:
		m.showHelp = true
	case keyRefresh:
		return m.refresh()
	case keyNew:
		return m.startCreate()
	case keyMenu:
		return m.openMenu(m.menuAnchorPoint()), nil
	case keyActions:
		return m.openActionsMenu(m.actionsAnchorPoint()), nil
	case keyToggleOutput:
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	case keyTab:
		return m.selectTab((m.tab + 1) % len(tabs))
	case keyShiftTab:
		return m.selectTab((m.tab + len(tabs) - 1) % len(tabs))
	case keyUp, keyVimUp:
		return m.moveCursor(-1), nil
	case keyDown, keyVimDown:
		return m.moveCursor(1), nil
	case keyPageUp:
		return m.moveCursor(-max(m.pageRows(layout), 1)), nil
	case keyPageDown:
		return m.moveCursor(max(m.pageRows(layout), 1)), nil
	case keyTop:
		return m.moveCursor(-m.rowCount()), nil
	case keyBottom:
		return m.moveCursor(m.rowCount()), nil
	case keyOutputUp:
		return m.scrollOutput(-1), nil
	case keyOutputDown:
		return m.scrollOutput(1), nil
	case keyEnter, keyRight, keyVimRight:
		if layout.Narrow {
			m.detailOpen = true
			return m.reflow(), nil
		}
	case keyEscape, keyLeft, keyVimLeft:
		if m.detailOpen {
			m.detailOpen = false
			return m.reflow(), nil
		}
	}

	return m, nil
}

// selectTab moves to a tab and, the first time the Tree tab is opened, asks for
// the forest it has never built.
func (m Model) selectTab(index int) (Model, tea.Cmd) {
	m.tab = index
	if index != tabTree || m.treeLoaded || m.treeLoading {
		return m.reflow(), nil
	}
	m.treeLoading = true
	return m.reflow(), m.loadTreeCmd()
}

func (m Model) pageRows(layout domain.DashboardLayout) int {
	if m.tab == tabTree {
		return layout.TreeRows
	}
	return layout.ListRows
}

func (m Model) inZone(id string, msg tea.MouseMsg) bool {
	return m.zones.Get(id).InBounds(msg)
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		return m, nil
	}
	if m.modal.open {
		return m.modalMouse(msg)
	}
	if m.menuOpen {
		return m.menuMouse(msg)
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.wheel(msg, -1), nil
	case tea.MouseButtonWheelDown:
		return m.wheel(msg, 1), nil
	}

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m.rightClick(msg)
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	for index := range tabs {
		if m.inZone(tabZone(index), msg) {
			return m.selectTab(index)
		}
	}

	if m.inZone(zoneOutputToggle, msg) {
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	}

	if m.inZone(zoneAdd, msg) {
		return m.startCreate()
	}

	if m.inZone(zoneActions, msg) {
		zone := m.zones.Get(zoneActions)
		return m.openActionsMenu(domain.Rect{X: zone.StartX, Y: zone.EndY}), nil
	}

	if model, hit := m.clickRow(msg); hit {
		return model, nil
	}

	return m, nil
}

// clickRow selects the row under the pointer on whichever tab is showing.
func (m Model) clickRow(msg tea.MouseMsg) (Model, bool) {
	if m.tab == tabTree {
		for index := range m.treeRows {
			if !m.inZone(treeRowZone(index), msg) {
				continue
			}
			m.treeCursor = index
			return m.reflow(), true
		}
		return m, false
	}
	for index := range m.statuses {
		if !m.inZone(rowZone(index), msg) {
			continue
		}
		m.cursor = index
		if m.layout().Narrow {
			m.detailOpen = true
		}
		return m.reflow(), true
	}
	return m, false
}

// menuMouse gives the menu the mouse while it is up: an entry activates, and
// anything else — a click beside it, the wheel, the right button again —
// dismisses it and does nothing more, as a context menu does everywhere.
func (m Model) menuMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button == tea.MouseButtonLeft {
		for index := range m.menuItems() {
			if m.inZone(menuZone(index), msg) {
				return m.activateMenu(index)
			}
		}
	}
	return m.closeMenu(), nil
}

// rightClick opens the context menu on the row it lands on, selecting it first:
// a menu that acted on another row than the one under the pointer would be a trap.
func (m Model) rightClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	model, hit := m.clickRow(msg)
	if !hit {
		return m, nil
	}
	return model.openMenu(domain.Rect{X: msg.X, Y: msg.Y}), nil
}

// modalMouse only ever resolves the modal's own rows: the frame behind it is
// still on the zone manager from the last frame that drew it.
func (m Model) modalMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	for index := range m.modal.rows {
		if !m.inZone(modalRowZone(index), msg) {
			continue
		}
		modal, cmd := m.modal.activate(index)
		m.modal = modal
		return m, cmd
	}
	return m, nil
}

func (m Model) wheel(msg tea.MouseMsg, delta int) Model {
	if m.outputExpanded && m.inZone(zoneOutput, msg) {
		return m.scrollOutput(delta)
	}
	if m.inZone(zoneList, msg) || m.inZone(zoneTree, msg) {
		return m.moveCursor(delta)
	}
	return m
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	layout := m.layout()
	body := ""
	switch {
	case layout.DetailVisible && layout.ListVisible:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderMain(layout), m.renderDetail(layout))
	case layout.DetailVisible:
		body = m.renderDetail(layout)
	default:
		body = m.renderMain(layout)
	}

	// A panel too small to draw returns nothing; keeping its empty line would push
	// the frame past the last row and make the alt screen scroll.
	sections := make([]string, 0, 4)
	for _, section := range []string{m.renderHeader(layout), body, m.renderOutput(layout), m.renderHelpBar(layout)} {
		if section != "" {
			sections = append(sections, section)
		}
	}

	return m.zones.Scan(m.withOverlays(lipgloss.JoinVertical(lipgloss.Left, sections...)))
}

// renderMain draws whichever tab owns the main panel. The Tree tab takes the
// list's place rather than the whole body, so the detail stays beside it and a
// node keeps leading somewhere.
func (m Model) renderMain(layout domain.DashboardLayout) string {
	if m.tab == tabTree {
		return m.renderTree(layout)
	}
	return m.renderList(layout)
}

// withOverlays pastes whatever is open over the frame. Only one ever is: each of
// them takes the keyboard while it is up.
func (m Model) withOverlays(frame string) string {
	if m.showHelp {
		box, rect := m.helpBox()
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	if m.modal.open {
		box, rect := m.modal.box(m.zones)
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	if m.menuOpen {
		box, rect := m.menuBox()
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	return frame
}
