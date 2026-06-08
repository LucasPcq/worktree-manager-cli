package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/styles"
)

// Indent is the standard left padding used by all TUI components.
// Use this to align non-TUI output with TUI prompts.
// The canonical definition lives in styles.Indent; this alias keeps callers unchanged.
const Indent = styles.Indent

// Warning prints a styled warning line: "  ! message".
func Warning(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.BadgeWarning.Render("!"), styles.Warning.Render(msg))
}

// InfoLine prints a styled key-value pair: "  label  value".
func InfoLine(w io.Writer, label string, value string) {
	fmt.Fprintf(w, "%s%s  %s\n", Indent, styles.Muted.Render(label), value)
}

// SectionTitle prints a bold section title with standard indent.
func SectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", Indent, styles.Bold.Render(title))
}

// Success prints a styled success line: "  ✓ message".
func Success(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Success.Render("✓"), msg)
}

// Error prints a styled error line: "  ✗ message".
// If msg contains newlines (e.g. captured subprocess output), only the first
// line rides next to the cross; the rest is indented underneath so terminal
// rendering stays readable.
func Error(w io.Writer, msg string) {
	cross := styles.DangerText.Render("✗")
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", Indent, cross, lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", Indent, styles.Muted.Render(line))
	}
}

// Loading prints a styled loading/status line: "  › message".
func Loading(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Muted.Render("›"), styles.Muted.Render(msg))
}

// Message prints a plain indented message.
func Message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", Indent, msg)
}

// Intro prints a styled introductory message used to open an interactive flow.
func Intro(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Primary.Render("›"), styles.Bold.Render(msg))
}

// Blank prints an empty line for vertical spacing.
func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// Callout prints a bordered notice box with a bold title followed by body lines.
// Use it to surface an optional, non-blocking hint above an interactive flow.
func Callout(w io.Writer, title string, lines []string) {
	rows := append([]string{styles.CalloutTitle.Render(title)}, lines...)
	box := styles.Callout.Render(strings.Join(rows, "\n"))
	fmt.Fprintf(w, "\n%s\n", box)
}

// AnnounceItem is a label-value pair displayed inside an Announce block.
type AnnounceItem struct {
	Label string
	Value string
}

// Announce prints a padded block with a blank line above, a bold section title,
// indented key-value rows, and a blank line below. Use it before an interactive
// picker to describe what is about to happen.
func Announce(w io.Writer, title string, items []AnnounceItem) {
	Blank(w)
	SectionTitle(w, title)
	for _, item := range items {
		InfoLine(w, item.Label, item.Value)
	}
	Blank(w)
}
