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

	// DashboardWordmark is the product's name in the header bar: the one place
	// the dashboard says what it is.
	DashboardWordmark = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Padding(0, 1)

	// DashboardTabActive and DashboardTabInactive render the tab bar entries. The
	// active one is named by weight and by the rule under it, not by a filled
	// block: a background that loud belongs to what the user is acting on.
	DashboardTabActive = lipgloss.NewStyle().Bold(true).Padding(0, 2)

	DashboardTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)

	// DashboardTabRule underlines the active tab, DashboardRule carries it across
	// the rest of the header.
	DashboardTabRule = lipgloss.NewStyle().Foreground(ColorPrimary)
	DashboardRule    = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardCount is the header's right-hand count.
	DashboardCount = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)

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

	// DashboardDanger renders what destroys something.
	DashboardDanger = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)

	// DashboardMenu frames the floating context menu, DashboardMenuTitle names
	// the worktree it acts on.
	DashboardMenu = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 1)

	DashboardMenuTitle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// DashboardAddButton is the header's call to action, and the one place a
	// filled block is warranted: it stands alone in a title row, with no focus
	// ring around it to say what it is.
	DashboardAddButton = lipgloss.NewStyle().
				Foreground(ColorBadgeFg).
				Background(ColorPrimary).
				Bold(true).
				Padding(0, 3)

	// DashboardDisabled renders what cannot be activated yet.
	DashboardDisabled = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// DashboardRowSelected tints the whole selected worktree row. Render plain
	// (ANSI-free) text through it so the background fills every cell.
	DashboardRowSelected = lipgloss.NewStyle().
				Background(ColorRowTint).
				Foreground(ColorSelectedFg)

	// DashboardRowBar is the accent bar down the left of the selected row,
	// DashboardRowName the worktree's own name — the heaviest thing in the list —
	// and DashboardRowMeta the quieter line under it.
	DashboardRowBar  = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DashboardRowName = lipgloss.NewStyle().Bold(true)
	DashboardRowMeta = lipgloss.NewStyle().Foreground(ColorMuted)
)
