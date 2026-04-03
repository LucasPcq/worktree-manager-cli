package dashboard

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

type pane int

const (
	paneList pane = iota
	paneDetail
	paneLogPanel
)

// NewParams holds inputs for creating a new dashboard model.
type NewParams struct {
	Config     domain.Config
	ProjectDir string
}

// Model is the main Bubbletea model for the wtm dashboard.
type Model struct {
	list       listModel
	detail     detailModel
	logPanel   logPanelModel
	logbar     logbarModel
	activePane pane
	config     domain.Config
	projectDir string
	program    *tea.Program
	width      int
	height     int
	keys       keyMap
	GoPath     string // set when user presses Enter — the caller prints this path for the shell wrapper
}

// New creates a new dashboard model.
func New(params NewParams) Model {
	return Model{
		config:     params.Config,
		projectDir: params.ProjectDir,
		activePane: paneList,
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
	return loadWorktrees(m.projectDir, m.config)
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
		return m, loadWorktrees(m.projectDir, m.config)

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

	listContent := m.list.view(m.activePane == paneList)
	listPanel := m.renderPanel("Worktrees", listContent, listWidth, contentHeight, m.activePane == paneList)

	// Each renderPanel border adds 2 lines (top + bottom) to the Height value.
	// So a panel with Height(h) actually renders as h+2 lines total.
	var rightPanel string
	if m.logPanel.visible {
		// Two panels: (detailH+2) + (logH+2) must equal contentHeight+2 (the single panel case)
		// So detailH + logH = contentHeight - 2
		availInner := contentHeight - 2
		detailHeight := availInner * 75 / 100
		logHeight := availInner - detailHeight

		detailPanel := m.renderPanel("Details", m.detail.viewport.View(), detailWidth, detailHeight, m.activePane == paneDetail)
		logsPanel := m.renderPanel("Hooks output", m.logPanel.view(), detailWidth, logHeight, m.activePane == paneLogPanel)

		rightPanel = lipgloss.JoinVertical(lipgloss.Left, detailPanel, logsPanel)
	} else {
		rightPanel = m.renderPanel("Details", m.detail.viewport.View(), detailWidth, contentHeight, m.activePane == paneDetail)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, rightPanel)
	logbar := m.logbar.view(m.width)

	return lipgloss.JoinVertical(lipgloss.Left, content, logbar)
}

func (m *Model) updateLayout() {
	listWidth := m.width * 30 / 100
	detailWidth := m.width - listWidth - 2
	contentHeight := m.height - 3
	innerHeight := max(1, contentHeight-4)
	m.list.setSize(listWidth-4, innerHeight)

	if m.logPanel.visible {
		availInner := contentHeight - 2
		detailHeight := availInner * 75 / 100
		logHeight := availInner - detailHeight
		m.detail.setSize(detailWidth-4, max(1, detailHeight-4))
		m.logPanel.setSize(detailWidth-4, max(1, logHeight-4))
	} else {
		m.detail.setSize(detailWidth-4, innerHeight)
	}
}

func (m *Model) availablePanes() []pane {
	panes := []pane{paneList, paneDetail}
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
	return paneList
}

func (m *Model) prevPane() pane {
	panes := m.availablePanes()
	for i, p := range panes {
		if p == m.activePane {
			return panes[(i-1+len(panes))%len(panes)]
		}
	}
	return paneList
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


	case key.Matches(msg, m.keys.Up):
		if m.activePane == paneList {
			m.list.moveUp()
			return m, m.triggerDetailLoad()
		}
		return m, m.scrollActiveViewport(msg)

	case key.Matches(msg, m.keys.Down):
		if m.activePane == paneList {
			m.list.moveDown()
			return m, m.triggerDetailLoad()
		}
		return m, m.scrollActiveViewport(msg)

	case key.Matches(msg, m.keys.Refresh):
		m.logbar.message = "Refreshing..."
		return m, loadWorktrees(m.projectDir, m.config)

	case key.Matches(msg, m.keys.Focus):
		selected, ok := m.list.selectedStatus()
		if !ok {
			return m, nil
		}
		m.logbar.message = fmt.Sprintf("Focusing on %s...", selected.Branch)
		m.logPanel.show()
		m.updateLayout()
		return m, focusWorktree(m.projectDir, selected.Branch, m.config, m.program)

	case key.Matches(msg, m.keys.New):
		if m.activePane != paneList {
			return m, nil
		}
		return m, execNewWorktree()

	case key.Matches(msg, m.keys.Delete):
		if m.activePane != paneList {
			return m, nil
		}
		selected, ok := m.list.selectedStatus()
		if !ok || selected.IsParent {
			return m, nil
		}
		return m, execCleanWorktree(selected.Branch)

	case key.Matches(msg, m.keys.Enter):
		if m.activePane != paneList {
			return m, nil
		}
		selected, ok := m.list.selectedStatus()
		if !ok {
			return m, nil
		}
		m.GoPath = selected.Path
		return m, tea.Quit
	}

	return m, nil
}

func (m *Model) handleWorktreeList(msg worktreeListMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logbar.message = fmt.Sprintf("Error: %v", msg.Err)
		return m, nil
	}

	m.list.setItems(msg.Statuses, msg.ActiveBranch)
	m.logbar.message = fmt.Sprintf("Loaded %d worktrees", len(msg.Statuses))
	return m, m.triggerDetailLoad()
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
