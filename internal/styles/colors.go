// Package styles centralizes all Lipgloss style definitions. No other package may instantiate lipgloss.Style.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	// ColorPrimary is the main accent color.
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#0F62FE", Dark: "#78A9FF"}

	// ColorMuted is used for secondary or deemphasized text.
	ColorMuted = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}

	// ColorSuccess indicates a positive state (clean, ok).
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#198038", Dark: "#42BE65"}

	// ColorWarning indicates an attention-needed state (dirty).
	ColorWarning = lipgloss.AdaptiveColor{Light: "#DA6D00", Dark: "#F1C21B"}

	// ColorDanger indicates a destructive or dangerous action.
	ColorDanger = lipgloss.AdaptiveColor{Light: "#DA1E28", Dark: "#FA4D56"}

	// ColorBadgeFg is the contrasted foreground for badge chips.
	ColorBadgeFg = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#161616"}

	// ColorSelectedBg is the background for the highlighted row in a list.
	ColorSelectedBg = lipgloss.AdaptiveColor{Light: "#D0E2FF", Dark: "#0043CE"}

	// ColorSelectedFg is the foreground for the highlighted row in a list.
	ColorSelectedFg = lipgloss.AdaptiveColor{Light: "#001D6C", Dark: "#FFFFFF"}
)
