package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

func renderPRDetail(pr domain.PRInfo, width int) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("  %s", styles.Bold.Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title))))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  %s %s", styles.Muted.Render("Author:"), pr.Author))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s", styles.Muted.Render("Opened:"), prAge(pr.CreatedAt)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s", styles.Muted.Render("Status:"), prStateLabel(pr)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s", styles.Muted.Render("Branch:"), pr.Branch))

	// Description
	if pr.Body != "" {
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s", styles.Bold.Render("Description")))
		b.WriteString("\n")
		wrapWidth := max(20, width-4) // 2 spaces indent on each side
		b.WriteString(indentLines(wrapText(pr.Body, wrapWidth), "  "))
	}

	// CI Status
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  %s", styles.Bold.Render("CI Status")))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s", ciLabel(pr.CIStatus)))

	// Reviews
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  %s", styles.Bold.Render("Reviews")))
	b.WriteString("\n")
	if len(pr.Reviews) == 0 {
		b.WriteString(styles.Muted.Render("  No reviews yet"))
	} else {
		for _, r := range pr.Reviews {
			b.WriteString(fmt.Sprintf("  %s %s — %s\n",
				reviewStateIcon(r.State),
				r.User,
				strings.ToLower(strings.ReplaceAll(r.State, "_", " ")),
			))
		}
	}

	return b.String()
}

func prStateLabel(pr domain.PRInfo) string {
	if pr.Draft {
		return styles.Muted.Render("draft")
	}
	return styles.CleanIndicator.Render("open")
}

func ciLabel(status domain.CIStatus) string {
	switch status {
	case domain.CIStatusPassing:
		return styles.CleanIndicator.Render("✓ All checks passing")
	case domain.CIStatusFailing:
		return styles.DirtyIndicator.Render("✗ Some checks failing")
	case domain.CIStatusPending:
		return styles.Warning.Render("… Checks in progress")
	default:
		return styles.Muted.Render("– No checks")
	}
}

// wrapText wraps long lines to fit within maxWidth, preserving existing newlines.
func wrapText(text string, maxWidth int) string {
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= maxWidth {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}
		for len(line) > 0 {
			if len(line) <= maxWidth {
				result.WriteString(line)
				result.WriteString("\n")
				break
			}
			// Find last space before maxWidth
			cut := strings.LastIndex(line[:maxWidth], " ")
			if cut <= 0 {
				cut = maxWidth
			}
			result.WriteString(line[:cut])
			result.WriteString("\n")
			line = strings.TrimLeft(line[cut:], " ")
		}
	}
	return strings.TrimRight(result.String(), "\n")
}

func reviewStateIcon(state string) string {
	switch state {
	case "APPROVED":
		return styles.CleanIndicator.Render("✓")
	case "CHANGES_REQUESTED":
		return styles.DirtyIndicator.Render("✗")
	case "COMMENTED":
		return styles.Muted.Render("●")
	default:
		return styles.Muted.Render("○")
	}
}
