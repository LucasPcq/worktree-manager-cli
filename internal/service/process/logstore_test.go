package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func readLog(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func TestJobLogPathSanitizesTheJobName(t *testing.T) {
	got := JobLogPath(JobLogPathParams{LogDir: "/state/logs/feat", Job: "web/api dev"})
	want := filepath.Join("/state/logs/feat", "web_api_dev.log")
	if got != want {
		t.Errorf("JobLogPath = %q, want %q", got, want)
	}
}

func TestLogSinkWritesSanitizedTimestampedLines(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "feat")
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	sink.Write([]byte("\x1b[32mready\x1b[0m\r\n"))
	sink.Write([]byte("building 10%\rbuilding 100%\nhalf a "))
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	lines := strings.Split(strings.TrimRight(readLog(t, filepath.Join(dir, "web.log")), "\n"), "\n")
	want := []string{"ready", "building 100%", "half a"}
	if len(lines) != len(want) {
		t.Fatalf("log has %d lines (%v), want %d", len(lines), lines, len(want))
	}
	for i, line := range lines {
		stamp, text, found := strings.Cut(line, domain.JobLogSeparator)
		if !found {
			t.Fatalf("line %q carries no timestamp", line)
		}
		if _, err := time.Parse(domain.JobLogTimestampLayout, stamp); err != nil {
			t.Errorf("line %q: %v", line, err)
		}
		if text != want[i] {
			t.Errorf("line %d = %q, want %q", i, text, want[i])
		}
	}
}

func TestLogSinkAppendsToAnExistingLog(t *testing.T) {
	dir := t.TempDir()

	first, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open first sink: %v", err)
	}
	first.Write([]byte("run one\n"))
	first.Close()

	second, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open second sink: %v", err)
	}
	second.Write([]byte("run two\n"))
	second.Close()

	content := readLog(t, filepath.Join(dir, "web.log"))
	if !strings.Contains(content, "run one") || !strings.Contains(content, "run two") {
		t.Errorf("a restarted job should extend its log, got:\n%s", content)
	}
}

func TestLogSinkRotatesAtTheThresholdAndKeepsMaxFiles(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 200})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	// Each line is ~30 bytes once stamped, so this crosses the threshold several
	// times and must never leave more than domain.JobLogMaxFiles files behind.
	for i := 0; i < 60; i++ {
		sink.Write([]byte("line\n"))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != domain.JobLogMaxFiles {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("log dir holds %v, want %d files", names, domain.JobLogMaxFiles)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", entry.Name(), err)
		}
		if info.Size() > 200+64 {
			t.Errorf("%s is %d bytes, want it bounded by the rotation threshold", entry.Name(), info.Size())
		}
	}
}

func TestTailJobLogReturnsTheLastLines(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	for i := 0; i < 10; i++ {
		sink.Write([]byte("line " + string(rune('0'+i)) + "\n"))
	}
	sink.Close()

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 3})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("tail returned %d lines, want 3", len(lines))
	}
	if !strings.HasSuffix(lines[2], "line 9") {
		t.Errorf("last line = %q, want the most recent one", lines[2])
	}
}

func logTexts(t *testing.T, lines []string) []string {
	t.Helper()
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		stamp, text, found := strings.Cut(line, domain.JobLogSeparator)
		if !found {
			t.Fatalf("line %q carries no timestamp", line)
		}
		if _, err := time.Parse(domain.JobLogTimestampLayout, stamp); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		out = append(out, text)
	}
	return out
}

// writeNumberedLines fills a job log with `count` lines, rotating every four.
func writeNumberedLines(t *testing.T, dir string, count int) {
	t.Helper()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 120})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	for i := 0; i < count; i++ {
		sink.Write([]byte(fmt.Sprintf("line %d\n", i)))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
}

func TestTailJobLogReachesIntoTheRotatedBackups(t *testing.T) {
	dir := t.TempDir()
	writeNumberedLines(t, dir, 8)

	backup := readLog(t, filepath.Join(dir, "web.log.1"))
	if !strings.Contains(backup, "line 2") {
		t.Fatalf("the fixture no longer rotates line 2 out of the active file:\n%s", backup)
	}
	if strings.Contains(readLog(t, filepath.Join(dir, "web.log")), "line 2") {
		t.Fatal("the fixture leaves line 2 in the active file, so the tail proves nothing")
	}

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 6})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}

	want := []string{"line 2", "line 3", "line 4", "line 5", "line 6", "line 7"}
	if got := logTexts(t, lines); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tail = %v, want %v — the two oldest come from the rotated backup", got, want)
	}
}

func TestTailJobLogSpansEveryRotatedFileInOrder(t *testing.T) {
	dir := t.TempDir()
	writeNumberedLines(t, dir, 12)

	for _, name := range []string{"web.log", "web.log.1", "web.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("the fixture did not produce %s: %v", name, err)
		}
	}

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 12})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}

	want := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		want = append(want, fmt.Sprintf("line %d", i))
	}
	if got := logTexts(t, lines); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tail = %v, want the three files read oldest first: %v", got, want)
	}
}

func TestTailJobLogOnAMissingFile(t *testing.T) {
	lines, err := TailJobLog(TailParams{LogDir: t.TempDir(), Job: "never-ran", Lines: 10})
	if err != nil {
		t.Fatalf("tail on a missing log should not fail: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("lines = %v, want none", lines)
	}
}

func TestPurgeWorktreeLogsIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "feat")
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	sink.Write([]byte("something\n"))
	sink.Close()

	if err := PurgeWorktreeLogs(dir); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("log dir still there after the purge (%v)", err)
	}
	if err := PurgeWorktreeLogs(dir); err != nil {
		t.Errorf("purging an absent log dir should succeed, got %v", err)
	}
	if err := PurgeWorktreeLogs(""); err != nil {
		t.Errorf("purging an unresolved log dir should succeed, got %v", err)
	}
}

func TestLogSinkBoundsTheTailOfALineThatNeverEnds(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	for i := 0; i < 20000; i++ {
		sink.Write([]byte("\r\x1b[K[" + strings.Repeat("=", i%20) + "] building packages"))
	}

	sink.mu.Lock()
	held := len(sink.pending)
	sink.mu.Unlock()
	if held > 64 {
		t.Errorf("the sink holds %d bytes of unterminated line, want only the last frame", held)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 10})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("log holds %d lines, want the redraw's last frame only", len(lines))
	}
	if !strings.HasSuffix(lines[0], "[===================] building packages") {
		t.Errorf("line = %q, want the last frame of the redraw", lines[0])
	}
}

func TestLogSinkRetiresARecordBiggerThanTheThreshold(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 100})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	sink.Write([]byte(strings.Repeat("x", 50000) + "\n"))

	active := filepath.Join(dir, "web.log")
	info, err := os.Stat(active)
	if err != nil {
		t.Fatalf("stat active log: %v", err)
	}
	if info.Size() > 100 {
		t.Errorf("active log is %d bytes right after an oversized record, want it bounded by the 100-byte threshold", info.Size())
	}
	if _, err := os.Stat(active + ".1"); err != nil {
		t.Errorf("the oversized record was not retired to a backup: %v", err)
	}

	sink.Write([]byte("after\n"))
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	if !strings.Contains(readLog(t, active), "after") {
		t.Error("the line written after the oversized one is missing from the active log")
	}
}
