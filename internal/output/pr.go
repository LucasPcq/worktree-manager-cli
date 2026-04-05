package output

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// PrintPRList writes a compact PR list to w.
func PrintPRList(prs []domain.PRInfo, w io.Writer) {
	if len(prs) == 0 {
		fmt.Fprintln(w, "No open pull requests.")
		return
	}

	for _, pr := range prs {
		fmt.Fprintln(w, formatPRLine(pr))
	}
}

func formatPRLine(pr domain.PRInfo) string {
	numberText := styles.Muted.Render(fmt.Sprintf("#%-4d", pr.Number))
	number := hyperlink(pr.URL, numberText)

	title := pr.Title
	if len(title) > 40 {
		title = title[:39] + "…"
	}

	branch := styles.Muted.Render(pr.Branch)

	author := styles.Muted.Render(pr.Author)

	age := formatAge(pr.CreatedAt)

	ci := formatCIStatus(pr.CIStatus)

	state := formatPRState(pr)

	open := hyperlink(pr.URL, styles.Primary.Render("Open"))

	return fmt.Sprintf("  %s  %-40s  %s  %s  %s  %s  %s  %s", number, title, branch, author, age, ci, state, open)
}

func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Hour:
		return styles.Muted.Render(fmt.Sprintf("%dm ago", int(d.Minutes())))
	case d < 24*time.Hour:
		return styles.Muted.Render(fmt.Sprintf("%dh ago", int(d.Hours())))
	default:
		days := int(math.Floor(d.Hours() / 24))
		return styles.Muted.Render(fmt.Sprintf("%dd ago", days))
	}
}

func formatCIStatus(status domain.CIStatus) string {
	switch status {
	case domain.CIStatusPassing:
		return styles.CleanIndicator.Render("CI ✓")
	case domain.CIStatusFailing:
		return styles.DirtyIndicator.Render("CI ✗")
	case domain.CIStatusPending:
		return styles.Warning.Render("CI …")
	default:
		return styles.Muted.Render("CI –")
	}
}

func formatPRState(pr domain.PRInfo) string {
	if pr.Draft {
		return styles.Muted.Render("draft")
	}

	approved := 0
	changesRequested := 0
	for _, r := range pr.Reviews {
		switch r.State {
		case "APPROVED":
			approved++
		case "CHANGES_REQUESTED":
			changesRequested++
		}
	}

	if changesRequested > 0 {
		return styles.DirtyIndicator.Render("changes requested")
	}
	if approved > 0 {
		return styles.CleanIndicator.Render(fmt.Sprintf("approved (%d)", approved))
	}

	reviewCount := len(pr.Reviews)
	if reviewCount > 0 {
		return styles.Muted.Render(fmt.Sprintf("%d review(s)", reviewCount))
	}

	return styles.Muted.Render("no reviews")
}

// FormatPRFilterLabel returns a human-readable label for a PR filter.
func FormatPRFilterLabel(filter domain.PRFilter) string {
	switch filter {
	case domain.PRFilterReviewRequested:
		return "Review requested"
	case domain.PRFilterMine:
		return "My PRs"
	default:
		return "All PRs"
	}
}

// FormatPRDetailSection renders a detailed view of a PR for the detail panel.
func FormatPRDetailSection(pr domain.PRInfo) string {
	var b strings.Builder

	// Header
	b.WriteString(styles.Bold.Render(fmt.Sprintf("#%d %s", pr.Number, pr.Title)))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render(fmt.Sprintf("  Author: %s  ·  %s  ·  %s", pr.Author, formatAge(pr.CreatedAt), pr.State)))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render(fmt.Sprintf("  Branch: %s", pr.Branch)))

	// CI status
	b.WriteString("\n\n")
	b.WriteString(styles.Bold.Render("CI Status"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s", formatCIStatus(pr.CIStatus)))

	// Reviews
	b.WriteString("\n\n")
	b.WriteString(styles.Bold.Render("Reviews"))
	b.WriteString("\n")
	if len(pr.Reviews) == 0 {
		b.WriteString(styles.Muted.Render("  No reviews yet"))
	} else {
		for _, r := range pr.Reviews {
			icon := reviewIcon(r.State)
			b.WriteString(fmt.Sprintf("  %s %s — %s\n", icon, r.User, strings.ToLower(strings.ReplaceAll(r.State, "_", " "))))
		}
	}

	return b.String()
}

// hyperlink wraps text in an OSC 8 terminal hyperlink sequence.
// Terminals that don't support it display the text unchanged.
func hyperlink(url, text string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func reviewIcon(state string) string {
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
