package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// JobLogPathParams holds inputs for resolving a job's log file.
type JobLogPathParams struct {
	LogDir string
	Job    string
}

// JobLogPath returns the active log file of a job inside its worktree's log dir.
func JobLogPath(params JobLogPathParams) string {
	return filepath.Join(params.LogDir, rules.JobLogFileName(params.Job))
}

// LogSinkParams holds inputs for opening a job's log file.
type LogSinkParams struct {
	LogDir string
	Job    string
	// MaxBytes overrides the rotation threshold; zero means domain.JobLogMaxBytes.
	MaxBytes int64
}

// LogSink persists a job's raw output as sanitized, timestamped lines, rotating
// the file once it outgrows its threshold. It is an io.Writer so it can sit
// beside the output hub on the job's drain path — a subscriber would have its
// chunks dropped under load, and a log that loses lines is not a log.
type LogSink struct {
	path     string
	maxBytes int64

	mu      sync.Mutex
	file    *os.File
	size    int64
	pending string
}

// OpenLogSink creates the log directory if needed and opens the job's log file
// in append mode, so a restarted job extends its history instead of erasing it.
func OpenLogSink(params LogSinkParams) (*LogSink, error) {
	if params.LogDir == "" {
		return nil, fmt.Errorf("log dir is required")
	}
	if err := os.MkdirAll(params.LogDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	path := JobLogPath(JobLogPathParams{LogDir: params.LogDir, Job: params.Job})
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open job log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat job log: %w", err)
	}

	return &LogSink{path: path, maxBytes: resolveMaxBytes(params.MaxBytes), file: file, size: info.Size()}, nil
}

func resolveMaxBytes(configured int64) int64 {
	if configured <= 0 {
		return domain.JobLogMaxBytes
	}
	return configured
}

// Write never fails and never blocks the caller on the file: the job's output
// matters more than its archive, so a broken sink stops writing and lets the
// PTY keep draining.
func (s *LogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return len(p), nil
	}
	result := rules.SanitizeLogChunk(rules.SanitizeChunkParams{
		Chunk:   string(p),
		Pending: s.pending,
		At:      time.Now(),
	})
	s.pending = result.Pending
	s.append(result.Records)
	return len(p), nil
}

// Close flushes the line the job left unterminated and releases the file.
func (s *LogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return nil
	}
	if text := rules.SanitizeLogLine(s.pending); text != "" {
		s.append([]domain.LogRecord{{At: time.Now(), Text: text}})
	}
	s.pending = ""

	file := s.file
	s.file = nil
	return file.Close()
}

func (s *LogSink) append(records []domain.LogRecord) {
	if len(records) == 0 {
		return
	}

	var buf strings.Builder
	for _, record := range records {
		buf.WriteString(rules.FormatLogRecord(record))
		buf.WriteByte('\n')
	}

	if s.size > 0 && s.size+int64(buf.Len()) > s.maxBytes {
		if err := s.rotate(); err != nil {
			s.disable()
			return
		}
	}

	written, err := s.file.WriteString(buf.String())
	s.size += int64(written)
	if err != nil {
		s.disable()
		return
	}

	// Rotating only before the write let a batch bigger than the threshold land
	// in the active file and stay there, growing it without bound. A record is
	// never split — a log line belongs to one file — so an oversized one is
	// written whole and retired immediately.
	if s.size >= s.maxBytes {
		if err := s.rotate(); err != nil {
			s.disable()
		}
	}
}

func (s *LogSink) rotate() error {
	if err := s.file.Close(); err != nil {
		return err
	}
	s.file = nil

	oldest := domain.JobLogMaxFiles - 1
	os.Remove(backupPath(s.path, oldest))
	for rank := oldest - 1; rank >= 1; rank-- {
		os.Rename(backupPath(s.path, rank), backupPath(s.path, rank+1))
	}
	if err := os.Rename(s.path, backupPath(s.path, 1)); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	s.file = file
	s.size = 0
	return nil
}

func (s *LogSink) disable() {
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
}

func backupPath(path string, rank int) string {
	return path + "." + strconv.Itoa(rank)
}

// TailParams holds inputs for reading the tail of a job's log.
type TailParams struct {
	LogDir string
	Job    string
	Lines  int
}

// TailJobLog returns the last lines persisted for a job, reaching into the
// rotated backups when the active file is too short to fill the request. A job
// that never wrote a log reads as no lines, not as an error.
func TailJobLog(params TailParams) ([]string, error) {
	if params.LogDir == "" || params.Lines <= 0 {
		return nil, nil
	}

	path := JobLogPath(JobLogPathParams{LogDir: params.LogDir, Job: params.Job})
	var lines []string
	for rank := 0; rank < domain.JobLogMaxFiles && len(lines) < params.Lines; rank++ {
		file := path
		if rank > 0 {
			file = backupPath(path, rank)
		}
		content, err := os.ReadFile(file)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read job log: %w", err)
		}
		lines = append(splitLogLines(content), lines...)
	}

	if len(lines) > params.Lines {
		return lines[len(lines)-params.Lines:], nil
	}
	return lines, nil
}

// PurgeWorktreeLogs deletes a worktree's whole log directory. Idempotent: an
// already-absent directory is a success.
func PurgeWorktreeLogs(logDir string) error {
	if logDir == "" {
		return nil
	}
	return os.RemoveAll(logDir)
}

func splitLogLines(content []byte) []string {
	trimmed := strings.TrimRight(string(content), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
