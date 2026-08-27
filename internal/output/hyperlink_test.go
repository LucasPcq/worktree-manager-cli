package output

import (
	"strings"
	"testing"
)

func TestHyperlink(t *testing.T) {
	on := Hyperlink(HyperlinkParams{Text: "http://x.localhost:4000", URL: "http://x.localhost:4000", Enabled: true})
	if !strings.HasPrefix(on, "\x1b]8;;") || !strings.Contains(on, "http://x.localhost:4000") {
		t.Errorf("enabled must wrap in OSC-8, got %q", on)
	}

	off := Hyperlink(HyperlinkParams{Text: "http://x.localhost:4000", URL: "http://x.localhost:4000"})
	if off != "http://x.localhost:4000" {
		t.Errorf("disabled must stay raw, got %q", off)
	}

	none := Hyperlink(HyperlinkParams{Text: "web", Enabled: true})
	if none != "web" {
		t.Errorf("no URL must stay raw, got %q", none)
	}
}
