// Package styles centralizes all Lipgloss style definitions. No other package may instantiate lipgloss.Style.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	// ColorPrimary carries navigation and selection: nothing else.
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5B4BE0", Dark: "#9B8CFF"}

	// ColorSignature dresses only the wordmark and the call to action. Its hue
	// is kept apart from ColorWarning (see colors_test.go): both are warm and
	// sit next to each other in the header.
	ColorSignature = lipgloss.AdaptiveColor{Light: "#A6391A", Dark: "#E8734A"}

	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#158A4A", Dark: "#4ADE80"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B07800", Dark: "#F0C000"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#C21E28", Dark: "#FA6E76"}

	ColorBadgeFg    = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#161616"}
	ColorSelectedBg = lipgloss.AdaptiveColor{Light: "#DCD7FB", Dark: "#3B2FA8"}
	ColorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}

	// ColorRowTint is the subtle background tint of the focused SelectList row,
	// paired with a colored left border + chevron. Lighter than ColorSelectedBg
	// so it marks the selection without overpowering the text.
	ColorRowTint = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}

	// ColorFlashTint is the brief highlight a just-created row's cursor lands
	// on before it settles into ColorRowTint: warmer and brighter, so the eye
	// finds it without a toast.
	ColorFlashTint = lipgloss.AdaptiveColor{Light: "#FBE4D8", Dark: "#4A2C1D"}
)
