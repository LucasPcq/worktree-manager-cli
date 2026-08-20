package styles

import "github.com/charmbracelet/lipgloss"

var (
	// RunViewPane frames a job's terminal emulator. No padding: every column
	// inside the border is one the job wrote to, and one the emulator was sized
	// for.
	RunViewPane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted)

	// RunViewSidebar frames the job list beside it.
	RunViewSidebar = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	// RunViewJobSelected marks the job whose pane is on screen.
	RunViewJobSelected = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
)
