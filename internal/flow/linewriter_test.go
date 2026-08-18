package flow

import (
	"strings"
	"testing"
)

// TestLineWriterEmitsWholeLines: a surface that consumes output as events must see
// each line once, whole, however the producer chopped its writes.
func TestLineWriterEmitsWholeLines(t *testing.T) {
	var got []string
	w := &LineWriter{Emit: func(line string) { got = append(got, line) }}

	writes := []string{"first", " line\nsec", "ond\n", "third\nfourth\n"}
	for _, chunk := range writes {
		if n, err := w.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", chunk, n, err, len(chunk))
		}
	}

	want := []string{"first line", "second", "third", "fourth"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("lines = %v, want %v", got, want)
	}
}

// TestLineWriterFlushEmitsThePartialLine: a hook whose last line has no newline
// must not be swallowed.
func TestLineWriterFlushEmitsThePartialLine(t *testing.T) {
	var got []string
	w := &LineWriter{Emit: func(line string) { got = append(got, line) }}

	if _, err := w.Write([]byte("done\ntail")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("lines = %v, want only the complete one before Flush", got)
	}

	w.Flush()
	if len(got) != 2 || got[1] != "tail" {
		t.Errorf("lines = %v, want the partial line flushed", got)
	}

	// Flushing again emits nothing: the buffer is empty.
	w.Flush()
	if len(got) != 2 {
		t.Errorf("lines = %v, want no duplicate on a second flush", got)
	}
}

func TestLineWriterWithoutEmitDiscards(t *testing.T) {
	w := &LineWriter{}
	if _, err := w.Write([]byte("ignored\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Flush()
}
