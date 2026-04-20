package styles

import "github.com/charmbracelet/lipgloss"

var (
	// DirtyIndicator is the style for "dirty" status indicators.
	DirtyIndicator = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// CleanIndicator is the style for "clean" status indicators.
	CleanIndicator = lipgloss.NewStyle().
			Foreground(ColorSuccess)
)
