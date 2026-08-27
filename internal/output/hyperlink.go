package output

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type HyperlinkParams struct {
	Text string
	URL  string
	// Enabled is false for a pipe, a JSON run, or a terminal that would only
	// show the escape sequence as garbage.
	Enabled bool
}

// Hyperlink wraps text in an OSC-8 sequence so the terminal makes it clickable,
// and returns it untouched everywhere else.
func Hyperlink(params HyperlinkParams) string {
	if !params.Enabled || params.URL == "" {
		return params.Text
	}
	return fmt.Sprintf(domain.HyperlinkFmt, params.URL, params.Text)
}
