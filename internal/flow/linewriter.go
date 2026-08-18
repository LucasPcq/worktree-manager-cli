package flow

import (
	"bytes"
	"strings"
)

// LineWriter turns a byte stream into whole lines, for a surface that consumes
// output as events rather than as a stream. Not concurrency-safe.
type LineWriter struct {
	Emit func(line string)
	buf  bytes.Buffer
}

func (w *LineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.Reset()
			w.buf.WriteString(line)
			return len(p), nil
		}
		w.emit(strings.TrimSuffix(line, "\n"))
	}
}

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
