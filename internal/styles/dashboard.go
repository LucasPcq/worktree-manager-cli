package styles

import "github.com/charmbracelet/lipgloss"

var (
	// DashboardPanel frames a dashboard panel. Callers set Width/Height; the
	// border and the title row are what DashboardChromeHeight accounts for.
	DashboardPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	// DashboardPanelTitle heads a panel body.
	DashboardPanelTitle = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	// DashboardTabActive and DashboardTabInactive render the tab bar entries.
	DashboardTabActive = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Background(ColorSelectedBg).
				Bold(true).
				Padding(0, 2)

	DashboardTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)

	// DashboardLabel names a detail field, DashboardValue carries it.
	DashboardLabel = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardValue = lipgloss.NewStyle()

	// DashboardBranch renders the detail panel's heading branch name.
	DashboardBranch = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// DashboardEmpty renders a panel's placeholder body.
	DashboardEmpty = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// DashboardClip carries no styling of its own; it exists so other packages can
	// hard-trim a rendered line to a cell width without instantiating a style.
	DashboardClip = lipgloss.NewStyle()

	// DashboardHelp renders the bottom key hint bar.
	DashboardHelp = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
)

var (
	// DashboardModal frames the box a flow's questions are asked in.
	DashboardModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// DashboardModalTitle heads the modal, DashboardModalHint closes it with the
	// keys that drive it.
	DashboardModalTitle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DashboardModalHint  = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardRow renders one interactive modal row, DashboardRowFocused the one
	// the keyboard is on.
	DashboardRow        = lipgloss.NewStyle()
	DashboardRowFocused = lipgloss.NewStyle().Foreground(ColorSelectedFg).Background(ColorSelectedBg)

	// DashboardDanger renders what destroys something, DashboardDisabled what
	// cannot be activated yet.
	DashboardDanger   = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	DashboardDisabled = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// DashboardAddButton is the list panel's header button.
	DashboardAddButton = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
)
