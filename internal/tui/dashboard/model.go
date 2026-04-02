package dashboard

import (
	"fmt"
	"strings"

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
	logbar     logbarModel
	activePane pane
	config     domain.Config
	projectDir string
	width      int
	height     int
	keys       keyMap
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

// Init starts the initial data load.
func (m Model) Init() tea.Cmd {
	return loadWorktrees(m.projectDir, m.config)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case focusDoneMsg:
		return m.handleFocusDone(msg)

	case logMsg:
		m.logbar.message = string(msg)
		return m, nil
	}

	// Forward scroll messages to viewport when detail is active
	if m.activePane == paneDetail {
		var cmd tea.Cmd
		m.detail.viewport, cmd = m.detail.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the dashboard.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	listWidth := m.width * 30 / 100
	detailWidth := m.width - listWidth - 2
	contentHeight := m.height - 3

	listContent := m.list.view(m.activePane == paneList)
	listPanel := m.renderPanel("Worktrees", listContent, listWidth, contentHeight, m.activePane == paneList)

	detailContent := m.detail.viewport.View()
	detailPanel := m.renderPanel("Details", detailContent, detailWidth, contentHeight, m.activePane == paneDetail)

	content := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)
	logbar := m.logbar.view(m.width)

	return lipgloss.JoinVertical(lipgloss.Left, content, logbar)
}

func (m *Model) updateLayout() {
	listWidth := m.width * 30 / 100
	detailWidth := m.width - listWidth - 2
	contentHeight := m.height - 3
	innerHeight := max(1, contentHeight-4)
	m.list.setSize(listWidth-4, innerHeight)
	m.detail.setSize(detailWidth-4, innerHeight)
}

func (m Model) renderPanel(title string, content string, width int, height int, focused bool) string {
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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Tab):
		if m.activePane == paneList {
			m.activePane = paneDetail
		} else {
			m.activePane = paneList
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.activePane == paneList {
			m.list.moveUp()
			return m, m.triggerDetailLoad()
		}
		// Forward to viewport
		var cmd tea.Cmd
		m.detail.viewport, cmd = m.detail.viewport.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Down):
		if m.activePane == paneList {
			m.list.moveDown()
			return m, m.triggerDetailLoad()
		}
		var cmd tea.Cmd
		m.detail.viewport, cmd = m.detail.viewport.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Refresh):
		m.logbar.message = "Refreshing..."
		return m, loadWorktrees(m.projectDir, m.config)

	case key.Matches(msg, m.keys.Focus):
		selected, ok := m.list.selectedStatus()
		if !ok {
			return m, nil
		}
		m.logbar.message = fmt.Sprintf("Focusing on %s...", selected.Branch)
		return m, focusWorktree(m.projectDir, selected.Branch, m.config)
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
		m.detail.lastError = strings.TrimSpace(msg.Output)
		m.detail.refreshContent()
		return m, nil
	}

	m.logbar.message = fmt.Sprintf("✓ Focused on %s", msg.Branch)
	m.detail.lastError = ""
	return m, loadWorktrees(m.projectDir, m.config)
}

func (m *Model) triggerDetailLoad() tea.Cmd {
	m.detail.lastError = ""
	m.logbar.message = ""

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
