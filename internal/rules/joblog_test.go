package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestStripTerminalEscapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"colors", "\x1b[32mready\x1b[0m", "ready"},
		{"line erase", "building\x1b[2K\x1b[1G", "building"},
		{"cursor move", "\x1b[1;1Hhome", "home"},
		{"private mode", "\x1b[?25lhidden\x1b[?25h", "hidden"},
		{"window title", "\x1b]0;vite dev\x07serving", "serving"},
		{"charset designation", "\x1b(Bplain", "plain"},
		{"single byte escape", "wrapped\x1bMnext", "wrappednext"},
		{"nothing to strip", "plain text", "plain text"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripTerminalEscapes(tc.raw); got != tc.want {
				t.Errorf("StripTerminalEscapes(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSanitizeLogLine(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"crlf", "done\r", "done"},
		{"progress redraw keeps the last frame", "[+] Running 1/3\r[+] Running 2/3\r[+] Running 3/3", "[+] Running 3/3"},
		{"colored redraw", "\x1b[33mbuild 10%\x1b[0m\r\x1b[32mbuild 100%\x1b[0m", "build 100%"},
		{"trailing spaces from a padded redraw", "ready       ", "ready"},
		{"stray control bytes", "bell\x07here", "bellhere"},
		{"tabs survive", "col1\tcol2", "col1\tcol2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeLogLine(tc.raw); got != tc.want {
				t.Errorf("SanitizeLogLine(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func texts(records []domain.LogRecord) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, record.Text)
	}
	return out
}

func TestSanitizeLogChunkSplitsCompleteLinesOnly(t *testing.T) {
	at := time.Date(2026, 8, 20, 12, 4, 11, 0, time.UTC)
	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: "first\nsecond\nthir", At: at})

	if got := texts(result.Records); strings.Join(got, "|") != "first|second" {
		t.Errorf("records = %v, want the two complete lines", got)
	}
	if result.Pending != "thir" {
		t.Errorf("pending = %q, want the incomplete tail", result.Pending)
	}
	for _, record := range result.Records {
		if !record.At.Equal(at) {
			t.Errorf("record stamped %v, want %v", record.At, at)
		}
	}
}

func TestSanitizeLogChunkJoinsALineSplitAcrossChunks(t *testing.T) {
	first := SanitizeLogChunk(SanitizeChunkParams{Chunk: "half of a "})
	if len(first.Records) != 0 {
		t.Fatalf("records = %v, want nothing before the newline", texts(first.Records))
	}

	second := SanitizeLogChunk(SanitizeChunkParams{Chunk: "line\n", Pending: first.Pending})
	if got := texts(second.Records); len(got) != 1 || got[0] != "half of a line" {
		t.Errorf("records = %v, want the rejoined line", got)
	}
	if second.Pending != "" {
		t.Errorf("pending = %q, want it drained", second.Pending)
	}
}

func TestSanitizeLogChunkJoinsAnEscapeSplitAcrossChunks(t *testing.T) {
	first := SanitizeLogChunk(SanitizeChunkParams{Chunk: "hello \x1b[3"})
	second := SanitizeLogChunk(SanitizeChunkParams{Chunk: "2mworld\x1b[0m\n", Pending: first.Pending})

	if got := texts(second.Records); len(got) != 1 || got[0] != "hello world" {
		t.Errorf("records = %v, want the escape stripped across the boundary", got)
	}
}

func TestSanitizeLogChunkHandlesCRLFAndProgressBars(t *testing.T) {
	raw := "\x1b[1mturbo\x1b[0m\r\n" +
		"downloading 10%\rdownloading 60%\rdownloading 100%\r\n" +
		"\r\n" +
		"done\n"

	result := SanitizeLogChunk(SanitizeChunkParams{Chunk: raw})

	want := []string{"turbo", "downloading 100%", "done"}
	got := texts(result.Records)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("records = %v, want %v", got, want)
	}
}

func TestFormatLogRecord(t *testing.T) {
	record := domain.LogRecord{
		At:   time.Date(2026, 8, 20, 12, 4, 11, 0, time.FixedZone("CEST", 2*60*60)),
		Text: "listening on :3000",
	}

	if got := FormatLogRecord(record); got != "2026-08-20T10:04:11Z  listening on :3000" {
		t.Errorf("FormatLogRecord = %q, want a UTC-stamped line", got)
	}
}
