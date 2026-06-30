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

	// FilterCursor renders the block cursor at the end of the filter input as a
	// reverse-video cell, so the caret (and any trailing space) stays visible.
	FilterCursor = lipgloss.NewStyle().Reverse(true)

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

	// Callout renders a bordered notice box used to surface optional hints.
	Callout = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Padding(0, 1).
		MarginLeft(2)

	// CalloutTitle renders the emphasized first line of a Callout box.
	CalloutTitle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	// StatusBox renders a neutral bordered box for an interactive flow's async
	// status region (loading spinner, GitHub availability hint). The border is
	// muted; severity is conveyed by the inner content, not the box.
	StatusBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1).
			MarginLeft(2)
)
