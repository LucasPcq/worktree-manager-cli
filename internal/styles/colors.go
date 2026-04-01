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
)
