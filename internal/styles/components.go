package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Breadcrumb renders the step counter (e.g. "Step 1/3").
	Breadcrumb = lipgloss.NewStyle().Foreground(ColorMuted)

	// BreadcrumbActive renders the current step name.
	BreadcrumbActive = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// ListItemSelected is the full-width highlight for the focused row.
	// Callers must set .Width(terminalWidth) before rendering.
	ListItemSelected = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorSelectedFg)

	// ListItemNormal is the default row style (no background).
	ListItemNormal = lipgloss.NewStyle()

	// ListCursor renders the selection arrow.
	ListCursor = lipgloss.NewStyle().Foreground(ColorPrimary)

	// InputPrompt renders the ">" prefix for text inputs.
	InputPrompt = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// FilterPrompt renders the "/" prefix in filter mode.
	FilterPrompt = lipgloss.NewStyle().Foreground(ColorMuted)

	// Badge renders a neutral chip.
	Badge = lipgloss.NewStyle().
		Background(ColorMuted).
		Foreground(ColorBadgeFg).
		PaddingLeft(1).
		PaddingRight(1)

	// BadgeSuccess renders a positive chip.
	BadgeSuccess = lipgloss.NewStyle().
			Background(ColorSuccess).
			Foreground(ColorBadgeFg).
			PaddingLeft(1).
			PaddingRight(1)

	// BadgeWarning renders an attention chip.
	BadgeWarning = lipgloss.NewStyle().
			Background(ColorWarning).
			Foreground(ColorBadgeFg).
			PaddingLeft(1).
			PaddingRight(1)

	// BadgeDanger renders a destructive chip.
	BadgeDanger = lipgloss.NewStyle().
			Background(ColorDanger).
			Foreground(ColorBadgeFg).
			PaddingLeft(1).
			PaddingRight(1)

	// DangerText styles destructive option labels.
	DangerText = lipgloss.NewStyle().Foreground(ColorDanger)

	// Divider renders a horizontal separator line.
	Divider = lipgloss.NewStyle().Foreground(ColorMuted)

	// SummaryLine renders a collapsed previous-step summary.
	SummaryLine = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	// HelpBar renders the bottom help text.
	HelpBar = lipgloss.NewStyle().Foreground(ColorMuted)
)
