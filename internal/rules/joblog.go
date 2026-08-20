package rules

import (
	"regexp"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// terminalEscape matches what a PTY-backed job writes to drive the terminal:
// CSI (colors, cursor moves, line clears), OSC (window title), charset
// designation, and the single-byte escapes.
var terminalEscape = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?|\x1b\[[0-9;?:<=>]*[ -/]*[@-~]|\x1b[()*+#][0-9A-Za-z]|\x1b[@-Z\\-_]`)

// StripTerminalEscapes returns s without its terminal escape sequences.
func StripTerminalEscapes(s string) string {
	return terminalEscape.ReplaceAllString(s, "")
}

// SanitizeChunkParams carries one chunk of raw job output together with the
// incomplete tail the previous chunk left behind.
type SanitizeChunkParams struct {
	Chunk   string
	Pending string
	At      time.Time
}

// SanitizeChunkResult holds the lines this chunk completed, and the tail still
// waiting for its newline — feed it back as Pending on the next chunk.
type SanitizeChunkResult struct {
	Records []domain.LogRecord
	Pending string
}

// SanitizeLogChunk turns raw output into whole plain-text lines. The tail is
// carried over untouched, which is what makes a chunk boundary falling in the
// middle of an escape sequence — or of a line — harmless.
func SanitizeLogChunk(params SanitizeChunkParams) SanitizeChunkResult {
	segments := strings.Split(params.Pending+params.Chunk, "\n")

	result := SanitizeChunkResult{Pending: segments[len(segments)-1]}
	for _, segment := range segments[:len(segments)-1] {
		text := SanitizeLogLine(segment)
		if text == "" {
			continue
		}
		result.Records = append(result.Records, domain.LogRecord{At: params.At, Text: text})
	}
	return result
}

// SanitizeLogLine reduces one raw line to its readable form. A line redrawn
// over itself with carriage returns keeps only its last state: a progress bar
// leaves its final value in the log, not each of its frames.
func SanitizeLogLine(raw string) string {
	line := strings.TrimSuffix(raw, "\r")
	if last := strings.LastIndex(line, "\r"); last >= 0 {
		line = line[last+1:]
	}
	return strings.TrimRight(stripControlRunes(StripTerminalEscapes(line)), " \t")
}

// FormatLogRecord renders a record as one line of a job log file.
func FormatLogRecord(record domain.LogRecord) string {
	return record.At.UTC().Format(domain.JobLogTimestampLayout) + domain.JobLogSeparator + record.Text
}

func stripControlRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
