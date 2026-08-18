package flow

import (
	"bytes"
	"strings"
)

// LineWriter turns a byte stream into whole lines, for a surface that consumes
// output as events rather than as a stream — a TUI appending to a viewport. Hook
// output goes through it unchanged otherwise: the CLI hands the hooks its own
// stderr, so it needs no buffering at all.
//
// Writes are not concurrency-safe: hooks run sequentially on one goroutine.
type LineWriter struct {
	// Emit receives each complete line, without its trailing newline.
	Emit func(string)
	buf  bytes.Buffer
}

// Write buffers p and emits every complete line it contains.
func (w *LineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// No newline yet: put the partial line back and wait for the rest.
			w.buf.Reset()
			w.buf.WriteString(line)
			return len(p), nil
		}
		w.emit(strings.TrimSuffix(line, "\n"))
	}
}

// Flush emits whatever is left without a trailing newline. Call it once the
// producer is done.
func (w *LineWriter) Flush() {
	if w.buf.Len() == 0 {
		return
	}
	rest := w.buf.String()
	w.buf.Reset()
	w.emit(rest)
}

func (w *LineWriter) emit(line string) {
	if w.Emit != nil {
		w.Emit(line)
	}
}
