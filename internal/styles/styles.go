// Package styles centralizes all Lipgloss style definitions. No other package may instantiate lipgloss.Style.
package styles

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

// UseRendererOn switches the default lipgloss renderer to the given writer.
// Call this before rendering styled output when stdout is captured (e.g. by a
// shell command substitution), so color detection happens against a TTY stream.
func UseRendererOn(w io.Writer) {
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(w))
}
