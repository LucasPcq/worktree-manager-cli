package dashboard

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

type pane int

const (
	paneList pane = iota
	panePRList
	paneDetail
	paneLogPanel
)

type activeListType int

const (
	activeListWorktrees activeListType = iota
	activeListPRs
)

// NewParams holds inputs for creating a new dashboard model.
type NewParams struct {
	Config     domain.Config
	ProjectDir string
	WorkDir    string // actual cwd (may differ from ProjectDir in a worktree)
	InitialPR  *int   // if set, open dashboard focused on this PR number
}

// Model is the main Bubbletea model for the wtm dashboard.
type Model struct {
	list             listModel
	prList           prListModel
	detail           detailModel
	logPanel         logPanelModel
	logbar           logbarModel
	activePane       pane
	activeList       activeListType
	prDetail         *domain.PRInfo
	allPRs           []domain.PRInfo // all open PRs, independent of filter
	initialPR        *int           // if set, select this PR on first load
	config           domain.Config
	projectDir       string
	workDir          string
	program          *tea.Program
	serviceStatuses  []process.ServiceInfo
	width            int
	height           int
	keys             keyMap
	GoPath           string // set when user presses Enter — the caller prints this path for the shell wrapper
}

// New creates a new dashboard model.
func New(params NewParams) Model {
	activePane := paneList
	activeList := activeListWorktrees

	if params.InitialPR != nil {
		activePane = panePRList
		activeList = activeListPRs
	}

	return Model{
		config:     params.Config,
		projectDir: params.ProjectDir,
		workDir:    params.WorkDir,
		activePane: activePane,
		activeList: activeList,
		initialPR:  params.InitialPR,
		prList:     newPRListModel(),
		keys:       defaultKeys,
		logbar:     logbarModel{message: "Loading..."},
	}
}

// SetProgram stores the tea.Program reference for the tuiWriter.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init starts the initial data load.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		loadWorktrees(m.projectDir, m.config),
		loadServiceStatuses(),
		loadPRs(m.projectDir, m.prList.filter),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case worktreeListMsg:
		return m.handleWorktreeList(msg)

	case detailLoadedMsg:
		m.detail.setDetail(&msg.Detail)
		return m, nil

	case hookOutputMsg:
		m.logPanel.appendLine(msg.Line)
		return m, nil

	case focusDoneMsg:
		return m.handleFocusDone(msg)

	case actionDoneMsg:
		m.prList.loading = true
		return m, tea.Batch(loadWorktrees(m.projectDir, m.config), loadServiceStatuses(), loadPRs(m.projectDir, m.prList.filter))

	case serviceListMsg:
		if msg.Err == nil {
			m.serviceStatuses = msg.Services
			m.detail.serviceStatuses = msg.Services
			m.list.serviceStatuses = msg.Services
			m.detail.refreshContent()
		}
		return m, nil

	case servicesStartedMsg:
		if msg.Err != nil {
			m.logbar.message = fmt.Sprintf("Services failed: %v", msg.Err)
		} else {
			m.logbar.message = "✓ Services started"
		}
		return m, loadServiceStatuses()

	case prListMsg:
		if msg.Err != nil {
			m.prList.setError(msg.Err)
			if m.activeList == activeListPRs {
				m.detail.setContent(styles.Muted.Render("  " + msg.Err.Error()))
			}
			return m, nil
		}
		m.prList.setItems(msg.PRs)
		// Store all PRs when using the "all" filter for worktree cross-refs
		if m.prList.filter == domain.PRFilterAll {
			m.allPRs = msg.PRs
			m.list.prInfos = msg.PRs
		}
		// Select initial PR if set
		if m.initialPR != nil {
			for i, pr := range msg.PRs {
				if pr.Number == *m.initialPR {
					m.prList.cursor = i
					break
				}
			}
			m.initialPR = nil
		}
		// Update detail panel if PR list is active
		if m.activeList == activeListPRs {
			return m, m.triggerPRDetailLoad()
		}
		return m, nil

	case prDetailMsg:
		if msg.Err != nil {
			m.detail.lastError = fmt.Sprintf("PR detail: %v", msg.Err)
		} else {
			m.prDetail = &msg.PR
			m.detail.setContent(renderPRDetail(msg.PR, m.detail.width))
		}
		return m, nil

	case logMsg:
		m.logbar.message = string(msg)
		return m, nil
	}

	return m, nil
}

// View renders the dashboard.
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	listWidth := m.width * 30 / 100
	detailWidth := m.width - listWidth - 2
	contentHeight := m.height - 3

	// Left panel: split 50/50 between worktrees and PRs
	// Two panels stacked: (wtH+2) + (prH+2) must equal contentHeight+2
	// So wtH + prH = contentHeight - 2
	leftInnerHeight := contentHeight - 2
	wtHeight := leftInnerHeight / 2
	prHeight := leftInnerHeight - wtHeight

	wtContent := m.list.view(m.activePane == paneList)
	wtPanel := m.renderPanel("Worktrees", wtContent, listWidth, wtHeight, m.activePane == paneList)

	prTitle := fmt.Sprintf("PRs — %s", m.prList.filterLabel())
	prContent := m.prList.view(m.activePane == panePRList)
	prPanel := m.renderPanel(prTitle, prContent, listWidth, prHeight, m.activePane == panePRList)

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, wtPanel, prPanel)

	// Right panel: detail (+ optional log panel)
	var rightPanel string
	if m.logPanel.visible {
		availableInnerHeight := contentHeight - 2
		detailHeight := availableInnerHeight * 75 / 100
		logHeight := availableInnerHeight - detailHeight

		detailPanel := m.renderPanel("Details", m.detail.viewport.View(), detailWidth, detailHeight, m.activePane == paneDetail)
		logsPanel := m.renderPanel("Hooks output", m.logPanel.view(), detailWidth, logHeight, m.activePane == paneLogPanel)

		rightPanel = lipgloss.JoinVertical(lipgloss.Left, detailPanel, logsPanel)
	} else {
		rightPanel = m.renderPanel("Details", m.detail.viewport.View(), detailWidth, contentHeight, m.activePane == paneDetail)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	logbar := m.logbar.view(m.width, m.activeList)

	return lipgloss.JoinVertical(lipgloss.Left, content, logbar)
}

func (m *Model) updateLayout() {
	listWidth := m.width * 30 / 100
	detailWidth := m.width - listWidth - 2
	contentHeight := m.height - 3

	// Left panel: split 50/50
	leftInnerHeight := contentHeight - 2
	wtHeight := leftInnerHeight / 2
	prHeight := leftInnerHeight - wtHeight
	m.list.setSize(listWidth-4, max(1, wtHeight-4))
	m.prList.setSize(listWidth-4, max(1, prHeight-4))

	if m.logPanel.visible {
		availableInnerHeight := contentHeight - 2
		detailHeight := availableInnerHeight * 75 / 100
		logHeight := availableInnerHeight - detailHeight
		m.detail.setSize(detailWidth-4, max(1, detailHeight-4))
		m.logPanel.setSize(detailWidth-4, max(1, logHeight-4))
	} else {
		innerHeight := max(1, contentHeight-4)
		m.detail.setSize(detailWidth-4, innerHeight)
	}
}

func (m *Model) activeListPane() pane {
	if m.activeList == activeListPRs {
		return panePRList
	}
	return paneList
}

func (m *Model) availablePanes() []pane {
	panes := []pane{m.activeListPane(), paneDetail}
	if m.logPanel.visible {
		panes = append(panes, paneLogPanel)
	}
	return panes
}

func (m *Model) nextPane() pane {
	panes := m.availablePanes()
	for i, p := range panes {
		if p == m.activePane {
			return panes[(i+1)%len(panes)]
		}
	}
	return m.activeListPane()
}

func (m *Model) prevPane() pane {
	panes := m.availablePanes()
	for i, p := range panes {
		if p == m.activePane {
			return panes[(i-1+len(panes))%len(panes)]
		}
	}
	return m.activeListPane()
}

func (m *Model) renderPanel(title string, content string, width int, height int, focused bool) string {
	style := styles.PanelInactive
	if focused {
		style = styles.PanelActive
	}

	titleStr := styles.PanelTitle.Render(title)

	return style.
		Width(width - 2).
		Height(height).
		Render(titleStr + "\n\n" + content)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Escape closes log panel
	if msg.String() == "esc" && m.logPanel.visible {
		m.logPanel.hide()
		m.updateLayout()
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Tab):
		m.activePane = m.nextPane()
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab):
		m.activePane = m.prevPane()
		return m, nil

	case key.Matches(msg, m.keys.WorktreeList):
		m.activeList = activeListWorktrees
		m.activePane = paneList
		return m, m.triggerDetailLoad()

	case key.Matches(msg, m.keys.PRList):
		m.activeList = activeListPRs
		m.activePane = panePRList
		return m, m.triggerPRDetailLoad()

	case key.Matches(msg, m.keys.Up):
		switch m.activePane {
		case paneList:
			m.list.moveUp()
			return m, m.triggerDetailLoad()
		case panePRList:
			m.prList.moveUp()
			return m, m.triggerPRDetailLoad()
		default:
			return m, m.scrollActiveViewport(msg)
		}

	case key.Matches(msg, m.keys.Down):
		switch m.activePane {
		case paneList:
			m.list.moveDown()
			return m, m.triggerDetailLoad()
		case panePRList:
			m.prList.moveDown()
			return m, m.triggerPRDetailLoad()
		default:
			return m, m.scrollActiveViewport(msg)
		}

	case key.Matches(msg, m.keys.Refresh):
		m.logbar.message = "Refreshing..."
		m.prList.loading = true
		return m, tea.Batch(
			loadWorktrees(m.projectDir, m.config),
			loadPRs(m.projectDir, m.prList.filter),
		)
	}

	// Context-dependent keys based on active list
	if m.activePane == panePRList {
		return m.handlePRAction(msg)
	}

	// Worktree-context keys
	switch {
	case key.Matches(msg, m.keys.New):
		return m, execNewWorktree()

	case key.Matches(msg, m.keys.CreatePR):
		selected, ok := m.list.selectedStatus()
		if !ok {
			return m, nil
		}
		return m, execCreatePR(selected.Path)

	case key.Matches(msg, m.keys.Focus),
		key.Matches(msg, m.keys.Delete),
		key.Matches(msg, m.keys.Enter),
		key.Matches(msg, m.keys.ServicesUp),
		key.Matches(msg, m.keys.ServicesDown),
		key.Matches(msg, m.keys.AttachService):
		selected, ok := m.list.selectedStatus()
		if !ok {
			return m, nil
		}
		return m.handleWorktreeAction(msg, selected)
	}

	return m, nil
}

func (m *Model) handlePRAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.FilterPR):
		m.prList.cycleFilter()
		m.detail.setContent(styles.Muted.Render("  Loading PRs..."))
		return m, loadPRs(m.projectDir, m.prList.filter)

	case key.Matches(msg, m.keys.OpenBrowser):
		pr, ok := m.prList.selectedPR()
		if !ok {
			return m, nil
		}
		return m, openInBrowser(pr.URL)
	}

	return m, nil
}

func (m *Model) handleWorktreeAction(msg tea.KeyMsg, selected domain.WorktreeStatus) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Focus):
		m.logbar.message = fmt.Sprintf("Focusing on %s...", selected.Branch)
		m.logPanel.show()
		m.updateLayout()
		return m, focusWorktree(m.projectDir, selected.Branch, m.config, m.program)

	case key.Matches(msg, m.keys.Delete):
		if selected.IsParent {
			return m, nil
		}
		return m, execCleanWorktree(selected.Branch)

	case key.Matches(msg, m.keys.Enter):
		m.GoPath = selected.Path
		return m, tea.Quit

	case key.Matches(msg, m.keys.ServicesUp):
		if !hasServicesConfig(selected.Path) {
			m.logbar.message = "No .wtm/services.toml found in this worktree"
			return m, nil
		}
		m.logbar.message = fmt.Sprintf("Starting services in %s...", selected.Branch)
		return m, startServicesCmd(selected.Path)

	case key.Matches(msg, m.keys.ServicesDown):
		worktreeServices := filterServicesByWorkDir(m.serviceStatuses, selected.Path)
		if len(worktreeServices) == 0 {
			m.logbar.message = "No services running in this worktree"
			return m, nil
		}
		m.logbar.message = fmt.Sprintf("Stopping services in %s...", selected.Branch)
		return m, stopServicesCmd(selected.Path, m.serviceStatuses)

	case key.Matches(msg, m.keys.AttachService):
		worktreeServices := filterServicesByWorkDir(m.serviceStatuses, selected.Path)
		if len(worktreeServices) == 0 {
			m.logbar.message = "No services running in this worktree"
			return m, nil
		}
		for _, svc := range worktreeServices {
			if svc.Status == domain.ServiceStatusRunning {
				return m, attachServiceCmd(svc.Name)
			}
		}
		m.logbar.message = "No running services to attach to"
		return m, nil
	}

	return m, nil
}

func (m *Model) handleWorktreeList(msg worktreeListMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logbar.message = fmt.Sprintf("Error: %v", msg.Err)
		return m, nil
	}

	m.list.setItems(msg.Statuses, msg.ActiveBranch)
	m.prList.setWorktreeBranches(msg.Statuses)
	m.logbar.message = fmt.Sprintf("Loaded %d worktrees", len(msg.Statuses))

	if m.activeList == activeListWorktrees {
		return m, m.triggerDetailLoad()
	}
	return m, nil
}

func (m *Model) handleFocusDone(msg focusDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logbar.message = fmt.Sprintf("Focus failed: %v", msg.Err)
		m.logPanel.appendLine(fmt.Sprintf("\n✗ %v\n", msg.Err))
		return m, nil
	}

	m.logbar.message = fmt.Sprintf("✓ Focused on %s", msg.Branch)
	m.logPanel.appendLine("\n✓ All hooks completed successfully\n")
	return m, loadWorktrees(m.projectDir, m.config)
}

func (m *Model) scrollActiveViewport(msg tea.Msg) tea.Cmd {
	switch m.activePane {
	case paneDetail:
		var cmd tea.Cmd
		m.detail.viewport, cmd = m.detail.viewport.Update(msg)
		return cmd
	case paneLogPanel:
		var cmd tea.Cmd
		m.logPanel.viewport, cmd = m.logPanel.viewport.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) triggerDetailLoad() tea.Cmd {
	m.detail.lastError = ""
	m.logbar.message = ""
	m.logPanel.hide()
	m.updateLayout()

	selected, ok := m.list.selectedStatus()
	if !ok {
		m.detail.setDetail(nil)
		return nil
	}

	return loadDetail(worktree.DetailParams{
		WorktreePath: selected.Path,
		ProjectDir:   m.projectDir,
		Branch:       selected.Branch,
		BaseBranch:   m.config.Project.Worktrees.BaseBranch,
	})
}

func (m *Model) triggerPRDetailLoad() tea.Cmd {
	m.detail.lastError = ""

	pr, ok := m.prList.selectedPR()
	if !ok {
		m.detail.setContent(styles.Muted.Render("  No PR selected"))
		return nil
	}

	// Show basic info immediately, then load full detail
	m.detail.setContent(renderPRDetail(pr, m.detail.width))
	return loadPRDetail(m.projectDir, pr.Number)
}
