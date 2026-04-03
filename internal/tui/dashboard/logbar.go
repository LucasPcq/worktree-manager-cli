package dashboard

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/styles"
)

type logbarModel struct {
	message string
}

func (m logbarModel) view(width int) string {
	help := styles.Muted.Render("q quit · n new · d clean · f focus · enter go · r refresh · tab switch")

	if m.message == "" {
		return help + strings.Repeat(" ", max(0, width-len(help)))
	}

	// Right-align the log message
	gap := max(1, width-printableWidth(help)-printableWidth(m.message)-2)
	return help + strings.Repeat(" ", gap) + m.message
}

func printableWidth(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}
