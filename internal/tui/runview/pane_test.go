package runview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func newTestPane(t *testing.T) *Pane {
	t.Helper()
	return NewPane(PaneParams{Size: PaneSize{Cols: 20, Rows: 3}, ScrollbackSize: 10})
}

func TestPaneRenderKeepsTruecolor(t *testing.T) {
	p := newTestPane(t)
	if _, err := p.Write([]byte("\x1b[38;2;255;0;0mred\x1b[m")); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := p.Render()
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Fatalf("truecolor sequence dropped from %q", out)
	}
}

func TestPaneRenderGivesWideRunesTwoCells(t *testing.T) {
	p := newTestPane(t)
	if _, err := p.Write([]byte("漢a")); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, _, _ := strings.Cut(ansi.Strip(p.Render()), "\n")
	if got := ansi.StringWidth(line); got != 3 {
		t.Fatalf("width of %q = %d, want 3", line, got)
	}
}
