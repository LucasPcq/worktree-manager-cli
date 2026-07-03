package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// IntroBox renders a section-intro callout: a left accent bar that signals
	// "this is an explanation". Set .Width before rendering to wrap the body.
	IntroBox = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(ColorPrimary).
			PaddingLeft(2).
			MarginLeft(2)

	// IntroTitle renders the bold heading of an intro callout.
	IntroTitle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// IntroNote renders the secondary "Detected: …" line of an intro callout.
	IntroNote = lipgloss.NewStyle().Foreground(ColorSuccess).Italic(true)

	// RecapTitle renders the header of a final recap step as a filled pill, so the
	// recap reads immediately as the synthesis / action point — visually distinct
	// from a plain intro callout title.
	RecapTitle = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorBadgeFg).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)
)

// RenderRecap renders a final recap block: a filled "Review & confirm" header
// pill above a word-wrapped body (the selection recap + any ⚠ warnings), inside
// the same accent bar as an intro. The filled header sets it apart from an intro
// callout so it reads as the synthesis and action point.
func RenderRecap(p IntroParams) string {
	width := p.Width
	if width <= 0 {
		width = 80
	}
	// Reserve room for margin (2) + border (1) + left padding (2) + right slack.
	inner := width - 8
	if inner < 24 {
		inner = 24
	}
	title := p.Title
	if title == "" {
		title = "Review & confirm"
	}

	var b strings.Builder
	b.WriteString(RecapTitle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(p.Body)
	if p.Note != "" {
		b.WriteString("\n\n")
		b.WriteString(IntroNote.Render(p.Note))
	}

	return IntroBox.Width(inner).Render(b.String())
}

// IntroParams holds the inputs for RenderIntro.
type IntroParams struct {
	Width int
	Title string
	Body  string
	Note  string
}

// RenderIntro renders a section-intro callout: a bold title, a word-wrapped
// body, and an optional emphasized note, inside a left accent bar.
func RenderIntro(p IntroParams) string {
	width := p.Width
	if width <= 0 {
		width = 80
	}
	// Reserve room for margin (2) + border (1) + left padding (2) + right slack.
	inner := width - 8
	if inner < 24 {
		inner = 24
	}

	var b strings.Builder
	if p.Title != "" {
		b.WriteString(IntroTitle.Render(p.Title))
		b.WriteString("\n\n")
	}
	b.WriteString(p.Body)
	if p.Note != "" {
		b.WriteString("\n\n")
		b.WriteString(IntroNote.Render(p.Note))
	}

	return IntroBox.Width(inner).Render(b.String())
}
