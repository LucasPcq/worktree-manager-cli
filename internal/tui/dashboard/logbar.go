package dashboard

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/styles"
)

type logbarModel struct {
	message string
}

const (
	helpWorktrees = "q quit · w worktrees · p prs · n new · d clean · f focus · u up · x down · s logs · enter go · r refresh"
	helpPRs       = "q quit · w worktrees · p prs · f filter · o open · r refresh"
)

func (m logbarModel) view(width int, activeList activeListType) string {
	helpText := helpWorktrees
	if activeList == activeListPRs {
		helpText = helpPRs
	}
	help := styles.Muted.Render(helpText)

	if m.message == "" {
		return help + strings.Repeat(" ", max(0, width-printableWidth(help)))
	}

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
