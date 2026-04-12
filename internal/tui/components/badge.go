// Package components provides reusable Bubbletea UI primitives.
package components

import "github.com/LucasPcq/wtm/internal/styles"

// BadgeVariant determines the color scheme of a badge chip.
type BadgeVariant int

const (
	// BadgeNeutral renders a gray chip.
	BadgeNeutral BadgeVariant = iota
	// BadgeSuccess renders a green chip.
	BadgeSuccess
	// BadgeWarning renders an orange chip.
	BadgeWarning
	// BadgeDanger renders a red chip.
	BadgeDanger
)

// Badge is a small colored chip rendered inline.
type Badge struct {
	Text    string
	Variant BadgeVariant
}

// Render returns the styled badge string.
func (b Badge) Render() string {
	switch b.Variant {
	case BadgeSuccess:
		return styles.BadgeSuccess.Render(b.Text)
	case BadgeWarning:
		return styles.BadgeWarning.Render(b.Text)
	case BadgeDanger:
		return styles.BadgeDanger.Render(b.Text)
	default:
		return styles.Badge.Render(b.Text)
	}
}
