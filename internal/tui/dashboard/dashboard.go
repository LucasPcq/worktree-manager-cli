// Package dashboard renders `wtm ui`: a full-screen view of the repository's
// worktrees, and the surface the create and clean flows are run from. It draws
// and it asks; what a run does is internal/flow's business, never its own.
package dashboard

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// RunParams holds what the dashboard needs to keep itself fed. The PR loader is
// injected so the slow `gh` call stays a surface concern, as in the pickers.
type RunParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	PRLoader   worktreepicker.PRLoaderFunc
}

// OutputLineMsg appends one line to the bottom output panel. Every phase of a
// running flow — hook output included — reaches the panel through it.
type OutputLineMsg struct{ Text string }

type worktreesMsg struct {
	statuses []domain.WorktreeStatus
	parents  map[string]string
	err      error
}

type prsMsg struct {
	prs  []domain.PRInfo
	conn domain.GHConnection
}

type pollMsg struct{}

var tabs = []string{domain.DashboardTabWorktrees}

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

	prs       []domain.PRInfo
	ghConn    domain.GHConnection
	prsLoaded bool

	outputLines    []string
	outputOffset   int
	outputExpanded bool

	detailOpen bool
	showHelp   bool

	menuOpen   bool
	menuCursor int

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
		zones:   zone.New(),
		msgs:    make(chan tea.Msg, domain.DashboardMsgBuffer),
		ghConn:  domain.GHConnectionOK,
		loading: true,
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
	return tea.Batch(m.loadWorktreesCmd(false), m.loadPRsCmd(), pollCmd(), listenCmd(m.msgs))
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.DashboardPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

// loadWorktreesCmd re-lists the worktrees off the UI thread. fetch reaches the
// remote (the `r` key); the poll never does, so the origin badges stay a
// deliberate, user-triggered refresh.
func (m Model) loadWorktreesCmd(fetch bool) tea.Cmd {
	listParams, stateDir := m.listParams, m.params.StateDir
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
		return worktreesMsg{statuses: statuses, parents: parents}
	}
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
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case worktreesMsg:
		return m.applyWorktrees(msg), nil

	case prsMsg:
		m.prs, m.ghConn, m.prsLoaded = msg.prs, msg.conn, true
		return m, nil

	case pollMsg:
		if m.loading {
			return m, pollCmd()
		}
		m.loading = true
		return m, tea.Batch(m.loadWorktreesCmd(false), pollCmd())

	case OutputLineMsg:
		return m.appendOutput(msg), nil

	case flowMsg:
		model, cmd := m.applyFlow(msg.inner)
		return model, tea.Batch(cmd, listenCmd(m.msgs))

	case opDoneMsg:
		return m.finishOp(msg), nil

	case modalLoadedMsg, formReadyMsg:
		return m.updateModal(msg)
	}

	return m, nil
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
	m.outputOffset = rules.DashboardClampOffset(rules.DashboardOffsetParams{
		Offset:  m.outputOffset,
		Total:   len(m.outputLines),
		Visible: layout.OutputLines,
	})
	return m
}

func (m Model) selected() (domain.WorktreeStatus, bool) {
	if m.cursor < 0 || m.cursor >= len(m.statuses) {
		return domain.WorktreeStatus{}, false
	}
	return m.statuses[m.cursor], true
}

func (m Model) moveCursor(delta int) Model {
	m.cursor = rules.ClampIndex(m.cursor+delta, len(m.statuses))
	return m.reflow()
}

func (m Model) scrollOutput(delta int) Model {
	m.outputOffset = max(m.outputOffset+delta, 0)
	return m.reflow()
}

func (m Model) refresh() (Model, tea.Cmd) {
	m.loading, m.prsLoaded = true, false
	return m, tea.Batch(m.loadWorktreesCmd(true), m.loadPRsCmd())
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
		return m.openMenu(), nil
	case keyToggleOutput:
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	case keyTab:
		m.tab = (m.tab + 1) % len(tabs)
	case keyShiftTab:
		m.tab = (m.tab + len(tabs) - 1) % len(tabs)
	case keyUp, keyVimUp:
		return m.moveCursor(-1), nil
	case keyDown, keyVimDown:
		return m.moveCursor(1), nil
	case keyPageUp:
		return m.moveCursor(-max(layout.ListRows, 1)), nil
	case keyPageDown:
		return m.moveCursor(max(layout.ListRows, 1)), nil
	case keyTop:
		return m.moveCursor(-len(m.statuses)), nil
	case keyBottom:
		return m.moveCursor(len(m.statuses)), nil
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

	if m.menuOpen {
		for index := range m.menuItems() {
			if m.inZone(menuZone(index), msg) {
				return m.activateMenu(index)
			}
		}
		m = m.closeMenu()
	}

	for index := range tabs {
		if m.inZone(tabZone(index), msg) {
			m.tab = index
			return m, nil
		}
	}

	if m.inZone(zoneOutputToggle, msg) {
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	}

	if m.inZone(zoneAdd, msg) {
		return m.startCreate()
	}

	for index := range m.statuses {
		if !m.inZone(rowZone(index), msg) {
			continue
		}
		m.cursor = index
		if m.layout().Narrow {
			m.detailOpen = true
		}
		return m.reflow(), nil
	}

	return m, nil
}

// rightClick opens the context menu on the row it lands on, selecting it first:
// a menu that acted on another row than the one under the pointer would be a trap.
func (m Model) rightClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	for index := range m.statuses {
		if !m.inZone(rowZone(index), msg) {
			continue
		}
		m.cursor = index
		return m.reflow().openMenu(), nil
	}
	return m, nil
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
	if m.inZone(zoneList, msg) {
		return m.moveCursor(delta)
	}
	return m
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.showHelp {
		return m.renderHelpOverlay()
	}
	if m.modal.open {
		return m.zones.Scan(m.modal.view(m.zones))
	}

	layout := m.layout()
	body := ""
	switch {
	case layout.DetailVisible && layout.ListVisible:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(layout), m.renderDetail(layout))
	case layout.DetailVisible:
		body = m.renderDetail(layout)
	default:
		body = m.renderList(layout)
	}

	// A panel too small to draw returns nothing; keeping its empty line would push
	// the frame past the last row and make the alt screen scroll.
	sections := make([]string, 0, 4)
	for _, section := range []string{m.renderTabs(layout), body, m.renderOutput(layout), m.renderHelpBar(layout)} {
		if section != "" {
			sections = append(sections, section)
		}
	}

	return m.zones.Scan(lipgloss.JoinVertical(lipgloss.Left, sections...))
}
