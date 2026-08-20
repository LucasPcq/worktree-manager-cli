package rules

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeLogDirParams struct {
	StateDir string
	Branch   string
}

// WorktreeLogDir returns <state-dir>/logs/<encoded-branch>/. Resolved
// client-side and handed to the daemon, which must never resolve a git dir of
// its own. The branch keys the directory the same way <state-dir>/worktrees/
// does — the state dir is shared by every worktree of the repository, so two
// worktrees whose paths share a base name would otherwise interleave their
// writes and purge each other. A branch that would not survive as a single path
// segment resolves to nothing: this path feeds a recursive delete, so refusing
// beats guessing.
func WorktreeLogDir(params WorktreeLogDirParams) string {
	if params.StateDir == "" || params.Branch == "" {
		return ""
	}
	segment := EncodeBranchSegment(params.Branch)
	if !isSinglePathSegment(segment) {
		return ""
	}
	return filepath.Join(params.StateDir, domain.JobLogsDirName, segment)
}

func isSinglePathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	return !strings.ContainsAny(segment, `/\`)
}

// JobLogFileName returns the log file name of a job, with every character that
// could escape the log directory or confuse a shell folded to an underscore.
func JobLogFileName(job string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.' || r == '-' || r == '_':
			return r
		default:
			return '_'
		}
	}, job)
	return safe + domain.JobLogFileExt
}

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

// SanitizeLogChunk turns raw output into whole plain-text lines. Escape
// sequences are left in the carried-over tail, which is what makes a chunk
// boundary falling in the middle of one — or of a line — harmless. The tail is
// bounded on both counts a job can breach it: redraw frames are collapsed as
// they arrive, and a line that never ends is journaled once it grows past
// domain.JobLogMaxPendingBytes.
func SanitizeLogChunk(params SanitizeChunkParams) SanitizeChunkResult {
	segments := strings.Split(params.Pending+params.Chunk, "\n")

	result := SanitizeChunkResult{}
	for _, segment := range segments[:len(segments)-1] {
		result.Records = appendRecord(result.Records, params.At, SanitizeLogLine(segment))
	}

	pending := collapseRedraws(segments[len(segments)-1])
	if len(pending) <= domain.JobLogMaxPendingBytes {
		result.Pending = pending
		return result
	}
	result.Records = appendRecord(result.Records, params.At, SanitizeLogLine(pending))
	return result
}

// SanitizeLogLine reduces one raw line to its readable form.
func SanitizeLogLine(raw string) string {
	return strings.TrimRight(stripControlRunes(StripTerminalEscapes(collapseRedraws(raw))), " \t")
}

// collapseRedraws keeps only the last state of a line written over itself with
// carriage returns: a progress bar leaves its final value in the log, not each
// of its frames. A trailing \r is kept — it may be the CR of a CRLF whose LF is
// still in the next chunk.
func collapseRedraws(raw string) string {
	end := len(raw)
	if strings.HasSuffix(raw, "\r") {
		end--
	}
	last := strings.LastIndex(raw[:end], "\r")
	if last < 0 {
		return raw
	}
	return raw[last+1:]
}

func appendRecord(records []domain.LogRecord, at time.Time, text string) []domain.LogRecord {
	if text == "" {
		return records
	}
	return append(records, domain.LogRecord{At: at, Text: text})
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
