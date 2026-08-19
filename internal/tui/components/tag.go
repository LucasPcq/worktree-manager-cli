package components

import "github.com/LucasPcq/wtm/internal/domain"

// TagVariantOf maps a declared tone onto this package's palette. It lives here
// so the two surfaces that render tagged options cannot drift apart on what
// "danger" looks like.
func TagVariantOf(tone domain.Tone) TagVariant {
	switch tone {
	case domain.ToneWarning:
		return TagWarning
	case domain.ToneDanger:
		return TagDanger
	default:
		return TagNeutral
	}
}
