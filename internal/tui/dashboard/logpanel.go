package dashboard

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/LucasPcq/wtm/internal/styles"
)

type logPanelModel struct {
	lines    []string
	viewport viewport.Model
	visible  bool
	width    int
	height   int
}

func (m *logPanelModel) show() {
	m.visible = true
	m.lines = nil
	m.refreshContent()
}

func (m *logPanelModel) hide() {
	m.visible = false
	m.lines = nil
}

func (m *logPanelModel) appendLine(line string) {
	m.lines = append(m.lines, line)
	m.refreshContent()
	m.viewport.GotoBottom()
}

func (m *logPanelModel) setSize(width int, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
}

func (m *logPanelModel) refreshContent() {
	content := strings.Join(m.lines, "")
	if content == "" {
		content = styles.Muted.Render("  Waiting for hook output...")
	}
	m.viewport.SetContent(content)
}

func (m logPanelModel) view() string {
	return m.viewport.View()
}
