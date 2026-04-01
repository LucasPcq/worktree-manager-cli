package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Bold renders text in bold.
	Bold = lipgloss.NewStyle().Bold(true)

	// Muted renders deemphasized text.
	Muted = lipgloss.NewStyle().Foreground(ColorMuted)

	// Success renders positive-state text.
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)

	// Warning renders attention-needed text.
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)

	// Primary renders accent text.
	Primary = lipgloss.NewStyle().Foreground(ColorPrimary)
)
