package styles

import "github.com/charmbracelet/lipgloss"

var (
	// PanelActive is the border style for the focused panel.
	PanelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary)

	// PanelInactive is the border style for unfocused panels.
	PanelInactive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted)

	// NormalItem is the style for unselected list items.
	NormalItem = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// ActiveIndicator is the style for the "● active" badge in the list.
	ActiveIndicator = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// DirtyIndicator is the style for "dirty" status in the list.
	DirtyIndicator = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// CleanIndicator is the style for "clean" status in the list.
	CleanIndicator = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// PanelTitle is the style for panel header titles.
	PanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			PaddingLeft(1)

)
