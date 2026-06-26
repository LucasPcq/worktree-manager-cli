package output

import (
	"bytes"
	"testing"
)

func TestFrame_WrapsBodyInSingleTopAndBottomBlank(t *testing.T) {
	var buf bytes.Buffer
	Frame(&buf, func() {
		Success(&buf, "done")
	})

	// The body line carries styled bytes; assert the frame shape rather than the
	// exact styled content: exactly one leading blank and one trailing blank.
	got := buf.String()
	if got[0] != '\n' {
		t.Errorf("expected a single leading blank line, got %q", got)
	}
	if len(got) < 2 || got[len(got)-1] != '\n' || got[len(got)-2] != '\n' {
		t.Errorf("expected a trailing blank line, got %q", got)
	}
}

func TestFrame_NoStackedBlankLines(t *testing.T) {
	var buf bytes.Buffer
	Frame(&buf, func() {
		Message(&buf, "hello")
	})

	if got := buf.String(); containsTripleNewline(got) {
		t.Errorf("frame must not produce stacked blank lines, got %q", got)
	}
}

func TestFrameStartFrameEnd_EachEmitOneBlank(t *testing.T) {
	var buf bytes.Buffer
	FrameStart(&buf)
	FrameEnd(&buf)

	if got := buf.String(); got != "\n\n" {
		t.Errorf("expected FrameStart+FrameEnd to emit exactly two newlines, got %q", got)
	}
}

func containsTripleNewline(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == '\n' && s[i+1] == '\n' && s[i+2] == '\n' {
			return true
		}
	}
	return false
}
