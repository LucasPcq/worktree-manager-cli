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

// PrintPRDetail writes a detailed view of a PR using the standard output helpers.
func PrintPRDetail(w io.Writer, pr domain.PRInfo) {
	SectionTitle(w, fmt.Sprintf("#%d %s", pr.Number, pr.Title))
	Blank(w)
	InfoLine(w, "Author", pr.Author)
	InfoLine(w, "Branch", pr.Branch)
	InfoLine(w, "Status", pr.State)
	InfoLine(w, "Opened", formatAge(pr.CreatedAt))
	Blank(w)

	if pr.Body != "" {
		SectionTitle(w, "Description")
		Message(w, pr.Body)
		Blank(w)
	}

	SectionTitle(w, "CI Status")
	switch pr.CIStatus {
	case domain.CIStatusPassing:
		Success(w, "CI passing")
	case domain.CIStatusFailing:
		Error(w, "CI failing")
	case domain.CIStatusPending:
		Message(w, styles.Warning.Render("CI pending"))
	default:
		Message(w, styles.Muted.Render("CI unknown"))
	}
	Blank(w)

	SectionTitle(w, "Reviews")
	if len(pr.Reviews) == 0 {
		Message(w, styles.Muted.Render("No reviews yet"))
	} else {
		for _, r := range pr.Reviews {
			label := fmt.Sprintf("%s — %s", r.User, strings.ToLower(strings.ReplaceAll(r.State, "_", " ")))
			switch r.State {
			case "APPROVED":
				Success(w, label)
			case "CHANGES_REQUESTED":
				Error(w, label)
			default:
				Message(w, label)
			}
		}
	}
	Blank(w)
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
