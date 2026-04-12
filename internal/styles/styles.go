// Package styles centralizes all Lipgloss style definitions. No other package may instantiate lipgloss.Style.
package styles

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// UseRendererOn re-detects lipgloss's color profile against the given writer.
// Package-level style vars hold a pointer to the default renderer, so swapping
// the renderer wouldn't re-style them — we mutate the shared renderer instead.
// Call this before rendering styled output when stdout is captured (e.g. by a
// shell command substitution), so color detection uses a TTY stream.
func UseRendererOn(w io.Writer) {
	lipgloss.SetColorProfile(termenv.NewOutput(w).EnvColorProfile())
}
