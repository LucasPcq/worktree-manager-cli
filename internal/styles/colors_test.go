package styles

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// hueOf extracts the HSL hue (0-360) of a "#RRGGBB" hex. Test helper only:
// production code never needs to reason about hues.
func hueOf(t *testing.T, hex string) float64 {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("malformed hex: %q", hex)
	}
	channel := func(offset int) float64 {
		v, err := strconv.ParseUint(hex[offset:offset+2], 16, 8)
		if err != nil {
			t.Fatalf("malformed hex: %q", hex)
		}
		return float64(v) / 255
	}
	r, g, b := channel(1), channel(3), channel(5)
	maxC, minC := max(r, max(g, b)), min(r, min(g, b))
	delta := maxC - minC
	if delta == 0 {
		return 0
	}
	var hue float64
	switch maxC {
	case r:
		hue = 60 * (((g - b) / delta) + 6)
	case g:
		hue = 60 * (((b-r)/delta) + 2)
	default:
		hue = 60 * (((r-g)/delta) + 4)
	}
	for hue >= 360 {
		hue -= 360
	}
	return hue
}

func hueDistance(a, b float64) float64 {
	d := a - b
	if d < 0 {
		d = -d
	}
	if d > 180 {
		return 360 - d
	}
	return d
}

// minWarmSeparation is the hue gap below which two warm colors become
// confusable on a terminal.
const minWarmSeparation = 25

func TestSignatureAndWarningAreNotConfusable(t *testing.T) {
	pairs := []struct {
		theme            string
		signature, warn  string
	}{
		{"light", ColorSignature.Light, ColorWarning.Light},
		{"dark", ColorSignature.Dark, ColorWarning.Dark},
	}
	for _, pair := range pairs {
		t.Run(pair.theme, func(t *testing.T) {
			got := hueDistance(hueOf(t, pair.signature), hueOf(t, pair.warn))
			if got < minWarmSeparation {
				t.Errorf("signature %s and warning %s: hue gap %.0f°, minimum %d°",
					pair.signature, pair.warn, got, minWarmSeparation)
			}
		})
	}
}

func TestEveryColorDefinesBothThemes(t *testing.T) {
	colors := map[string]lipgloss.AdaptiveColor{
		"primary":   ColorPrimary,
		"signature": ColorSignature,
		"success":   ColorSuccess,
		"warning":   ColorWarning,
		"danger":    ColorDanger,
		"muted":     ColorMuted,
	}
	for name, color := range colors {
		if color.Light == "" || color.Dark == "" {
			t.Errorf("%s: both themes must be defined, got %+v", name, color)
		}
	}
}
