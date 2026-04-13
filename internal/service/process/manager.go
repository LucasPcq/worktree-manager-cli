// Package process manages long-running jobs with pseudo-terminals.
package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ansiCSI matches CSI-style ANSI escape sequences (colors, cursor moves, line
// clears — everything docker compose emits while drawing its progress block).
var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

// detachedOutputBufferSize bounds how much PTY output we keep around while
// waiting for a detached-service launcher (e.g. `docker compose up -d`) to
// exit. Enough for the typical Docker/compose error line.
const detachedOutputBufferSize = 8 * 1024

// outputHistoryBytes is the size of the per-job rolling output buffer that
// the daemon drains from each long-running PTY. Replayed to clients on attach.
const outputHistoryBytes = 64 * 1024

// defaultPTYRows and defaultPTYCols are the fallback PTY dimensions used when
// a job is spawned before any client has attached. TUI apps read the PTY size
// at startup — a 0x0 window makes them bail to plain log mode.
const (
	defaultPTYRows = 40
	defaultPTYCols = 120
)

// ManagedJob holds the state of a running job.
type ManagedJob struct {
	Name    string
	Config  domain.JobConfig
	Cmd     *exec.Cmd
	PTY     *os.File
	Status  domain.JobStatus
	PID     int
	WorkDir string
	output  *outputHub // nil for detached launcher-style services
}

// Manager tracks and controls running jobs.
type Manager struct {
	jobs map[string]*ManagedJob
	mu   sync.Mutex
}

// NewManager creates a new job manager.
func NewManager() *Manager {
	return &Manager{
		jobs: make(map[string]*ManagedJob),
	}
}

// jobKey returns a unique identifier for a job scoped to its worktree.
func jobKey(name string, workDir string) string {
	return workDir + ":" + name
}

// Start launches a job in a PTY. Behavior depends on the job's kind:
//
//   - Task: blocks until the command exits, streams output to `streamer` if
//     non-nil, and removes the job from the map whatever the outcome. Returns
//     an error on non-zero exit (with captured output).
//   - Service with Stop (detached): blocks until the launcher command exits,
//     captures output into a bounded buffer so compose failures surface to
//     the caller, and stays registered as Running afterwards.
//   - Service without Stop (foreground): returns immediately after pty.Start,
//     with a background goroutine draining output and watching for exit.
func (m *Manager) Start(job domain.JobConfig, workDir string, streamer io.Writer) error {
	key := jobKey(job.Name, workDir)

	parts := strings.Fields(job.Cmd)
	if len(parts) == 0 {
		return fmt.Errorf("job %s has empty cmd", job.Name)
	}

	m.mu.Lock()
	if existing, ok := m.jobs[key]; ok && existing.Status == domain.JobStatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already running", job.Name)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	if job.Cwd != "" && !filepath.IsAbs(job.Cwd) {
		cmd.Dir = filepath.Join(workDir, job.Cwd)
	} else if job.Cwd != "" {
		cmd.Dir = job.Cwd
	} else {
		cmd.Dir = workDir
	}
	cmd.Env = jobEnv(job.Kind)

	output, err := spawnJob(cmd, job.Kind)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start job %s: %w", job.Name, err)
	}

	managed := &ManagedJob{
		Name:    job.Name,
		Config:  job,
		Cmd:     cmd,
		PTY:     output,
		Status:  domain.JobStatusRunning,
		PID:     cmd.Process.Pid,
		WorkDir: workDir,
	}
	m.jobs[key] = managed
	m.mu.Unlock()

	switch {
	case job.Kind == domain.JobKindTask:
		return m.runTask(managed, streamer)
	case job.IsDetached():
		if err := m.waitDetached(managed); err != nil {
			m.mu.Lock()
			delete(m.jobs, key)
			m.mu.Unlock()
			return err
		}
		return nil
	default:
		managed.output = newOutputHub(outputHistoryBytes)
		go m.drainToHub(managed)
		go m.waitForExit(managed)
		return nil
	}
}

// spawnJob starts the command and returns the *os.File from which the child's
// merged stdout/stderr can be drained. Services run through a PTY (so TUI
// apps render properly); tasks run through a plain pipe (so isatty(stdout)
// returns false in the child and TUI tools auto-fall back to sequential log
// output instead of corrupting the user's terminal with cursor-control codes
// targeted at a fictional PTY size).
func spawnJob(cmd *exec.Cmd, kind domain.JobKind) (*os.File, error) {
	if kind == domain.JobKindTask {
		pr, pw, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("create pipe: %w", err)
		}
		cmd.Stdout = pw
		cmd.Stderr = pw
		if err := cmd.Start(); err != nil {
			pr.Close()
			pw.Close()
			return nil, err
		}
		// Close the parent's copy of the write end so the reader sees EOF
		// when the child closes its own stdout/stderr (i.e. when it exits).
		pw.Close()
		return pr, nil
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	// Initialize the PTY to a reasonable size before the child reads it.
	// TUI frameworks like Ink (turbo, pnpm dev) decide at startup whether to
	// render based on window size — a 0x0 PTY makes them fall back to plain
	// log mode permanently. The real size is re-synced when a client attaches.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: defaultPTYRows, Cols: defaultPTYCols})
	return ptmx, nil
}

// jobEnv returns the environment to use for spawned jobs. The daemon runs
// with Setsid so its own TTY-related env may be missing or degraded.
//
// Services run inside a PTY: we force a TUI-friendly env so frameworks like
// Ink (turbo dev, pnpm dev) render properly when a client attaches.
//
// Tasks run through a pipe and stream their output directly to the user's
// terminal. We tell them TERM=dumb so they don't emit cursor or alt-screen
// sequences (which would erase the output once the task exits) but keep
// FORCE_COLOR / COLORTERM so colored log output still works. CI=true is the
// universal "I'm in a non-interactive env, output plain logs" hint that
// turbo, jest, npm, etc. respect.
func jobEnv(kind domain.JobKind) []string {
	env := os.Environ()
	if kind == domain.JobKindTask {
		env = setEnv(env, "TERM", "dumb")
		if _, ok := lookupEnv(env, "COLORTERM"); !ok {
			env = append(env, "COLORTERM=truecolor")
		}
		if _, ok := lookupEnv(env, "FORCE_COLOR"); !ok {
			env = append(env, "FORCE_COLOR=1")
		}
		if _, ok := lookupEnv(env, "CI"); !ok {
			env = append(env, "CI=true")
		}
		return env
	}

	if _, ok := lookupEnv(env, "TERM"); !ok {
		env = append(env, "TERM=xterm-256color")
	}
	if _, ok := lookupEnv(env, "COLORTERM"); !ok {
		env = append(env, "COLORTERM=truecolor")
	}
	if _, ok := lookupEnv(env, "FORCE_COLOR"); !ok {
		env = append(env, "FORCE_COLOR=1")
	}
	return env
}

// setEnv returns env with the given key set to value, replacing any existing
// occurrence. Use this when the override must win over what the daemon
// inherited (e.g. TERM=dumb for tasks regardless of the user's shell TERM).
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return append(out, prefix+value)
}

func lookupEnv(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix), true
		}
	}
	return "", false
}

// runTask executes a one-shot task. Output is mirrored to the optional
// streamer (so the CLI can forward it to the user in real time) and kept in
// the job's output hub so `run logs <task>` can also attach while it runs.
// The job is removed from the map on exit, whatever the outcome.
func (m *Manager) runTask(job *ManagedJob, streamer io.Writer) error {
	key := jobKey(job.Name, job.WorkDir)

	job.output = newOutputHub(outputHistoryBytes)
	go m.drainToHub(job)

	var unsub func()
	if streamer != nil {
		_, ch, release, subErr := job.output.Subscribe()
		if subErr == nil {
			unsub = release
			go func() {
				for chunk := range ch {
					_, _ = streamer.Write(chunk)
				}
			}()
		}
	}

	waitErr := job.Cmd.Wait()
	_ = job.PTY.Close()

	// Snapshot the captured output BEFORE closing the hub — on failure we
	// want to embed it in the error so the CLI can surface "why it failed"
	// without the user having to run `run logs` (the job will be gone).
	var captured string
	if waitErr != nil {
		job.output.mu.Lock()
		captured = strings.TrimSpace(string(job.output.history.Snapshot()))
		job.output.mu.Unlock()
	}

	job.output.close()
	if unsub != nil {
		unsub()
	}

	m.mu.Lock()
	delete(m.jobs, key)
	m.mu.Unlock()

	if waitErr != nil {
		if captured != "" {
			return fmt.Errorf("task %s failed:\n%s", job.Name, captured)
		}
		return fmt.Errorf("task %s failed: %w", job.Name, waitErr)
	}
	return nil
}

// waitDetached drains PTY output into a bounded buffer and blocks until the
// launcher process exits. On non-zero exit, returns an error containing the
// captured output so the user sees what went wrong. On success, the job
// stays registered as Running (the real work is detached).
func (m *Manager) waitDetached(job *ManagedJob) error {
	buf := newRingBuffer(detachedOutputBufferSize)
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, job.PTY)
		close(drained)
	}()

	err := job.Cmd.Wait()

	// Closing the PTY unblocks the drain goroutine in case Copy is still
	// reading; on normal exit it returns on its own.
	_ = job.PTY.Close()
	<-drained

	if err != nil {
		out := cleanPTYOutput(buf.String())
		if out == "" {
			return fmt.Errorf("job %s failed: %w", job.Name, err)
		}
		return fmt.Errorf("job %s failed:\n%s", job.Name, out)
	}
	return nil
}

// cleanPTYOutput strips ANSI escape sequences and collapses carriage-return
// progress redraws (docker compose writes `[+] Running 1/2\r[+] Running 2/2`)
// into distinct lines, then keeps only non-empty trimmed lines.
func cleanPTYOutput(raw string) string {
	raw = ansiCSI.ReplaceAllString(raw, "")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

// ringBuffer keeps only the most recent `cap` bytes written to it.
type ringBuffer struct {
	buf []byte
	cap int
}

func newRingBuffer(cap int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, 0, cap), cap: cap}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.cap {
		r.buf = r.buf[len(r.buf)-r.cap:]
	}
	return len(p), nil
}

func (r *ringBuffer) String() string { return string(r.buf) }

// Snapshot returns a copy of the buffer's current contents.
func (r *ringBuffer) Snapshot() []byte {
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}

// outputHub fans PTY output out to a rolling history buffer and any attached
// subscribers. One job → one hub → at most one subscriber at a time (matches
// wtm's single-attach model).
type outputHub struct {
	mu      sync.Mutex
	history *ringBuffer
	sub     chan []byte
	closed  bool
}

func newOutputHub(capacity int) *outputHub {
	return &outputHub{history: newRingBuffer(capacity)}
}

// Write records data into the history buffer and forwards a copy to the
// current subscriber if any. It never blocks on the subscriber: the subscribe
// channel is large enough to absorb normal bursts, and a full channel means
// the client has disappeared — we drop silently rather than stall the PTY
// reader.
func (h *outputHub) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history.Write(p)
	if h.sub != nil {
		data := make([]byte, len(p))
		copy(data, p)
		select {
		case h.sub <- data:
		default:
		}
	}
	return len(p), nil
}

// Subscribe registers a subscriber. Returns the current history snapshot and a
// channel streaming subsequent writes. At most one subscriber is allowed at a
// time — returns an error if one is already attached. The returned unsubscribe
// func must be called to release the slot.
func (h *outputHub) Subscribe() (history []byte, ch <-chan []byte, unsub func(), err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, nil, nil, fmt.Errorf("job output closed")
	}
	if h.sub != nil {
		return nil, nil, nil, fmt.Errorf("job already has a subscriber")
	}
	history = h.history.Snapshot()
	subCh := make(chan []byte, 256)
	h.sub = subCh
	unsub = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.sub == subCh {
			h.sub = nil
			close(subCh)
		}
	}
	return history, subCh, unsub, nil
}

// close releases the current subscriber (if any) and marks the hub as closed
// so new subscribers are rejected.
func (h *outputHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	if h.sub != nil {
		close(h.sub)
		h.sub = nil
	}
}

// drainToHub copies PTY output into the job's output hub until the PTY
// closes. Long-running jobs need a continuous reader so the OS PTY buffer
// never fills up (which would block the job's writes before any client
// attaches).
func (m *Manager) drainToHub(job *ManagedJob) {
	_, _ = io.Copy(job.output, job.PTY)
	job.output.close()
}

// Stop stops a job by name and workDir.
func (m *Manager) Stop(name string, workDir string) error {
	return m.stopByKey(jobKey(name, workDir))
}

// StopAll stops every running job across every worktree. Intended for daemon
// shutdown; callers that want to stop only one worktree's jobs must use
// StopAllInWorkDir.
func (m *Manager) StopAll() error {
	return m.stopAllMatching(func(*ManagedJob) bool { return true })
}

// StopAllInWorkDir stops every running job attached to the given workDir.
// Jobs belonging to other worktrees are left untouched.
func (m *Manager) StopAllInWorkDir(workDir string) error {
	return m.stopAllMatching(func(job *ManagedJob) bool {
		return job.WorkDir == workDir
	})
}

func (m *Manager) stopAllMatching(keep func(*ManagedJob) bool) error {
	m.mu.Lock()
	keys := make([]string, 0, len(m.jobs))
	for key, job := range m.jobs {
		if job.Status != domain.JobStatusRunning {
			continue
		}
		if !keep(job) {
			continue
		}
		keys = append(keys, key)
	}
	m.mu.Unlock()

	var firstErr error
	for _, key := range keys {
		if err := m.stopByKey(key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) stopByKey(key string) error {
	m.mu.Lock()
	job, ok := m.jobs[key]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("job not found")
	}

	// Always run the stop command if configured — handles detached processes
	// like "docker compose up -d" where the launcher exits but services keep
	// running.
	if job.Config.Stop != "" {
		return m.stopWithCommand(job)
	}

	if job.Status != domain.JobStatusRunning {
		return nil
	}

	return m.stopWithSignal(job)
}

// AttachSession holds everything a client needs to stream a job's output:
// the PTY (for stdin forwarding and window-size ioctls), the history to
// replay, and a channel delivering subsequent output. The caller must invoke
// Release when done.
type AttachSession struct {
	PTY     *os.File
	History []byte
	Stream  <-chan []byte
	Release func()
}

// Attach subscribes to a job's output hub and returns a session the daemon
// can use to stream history + live output to a client while still forwarding
// the client's stdin back into the PTY.
func (m *Manager) Attach(name string, workDir string) (*AttachSession, error) {
	key := jobKey(name, workDir)
	m.mu.Lock()
	job, ok := m.jobs[key]
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("job %s not found", name)
	}
	if job.Status != domain.JobStatusRunning {
		return nil, fmt.Errorf("job %s is not running", name)
	}
	if job.output == nil {
		return nil, fmt.Errorf("job %s has no attachable output (detached launcher)", name)
	}

	history, stream, unsub, err := job.output.Subscribe()
	if err != nil {
		return nil, err
	}
	return &AttachSession{
		PTY:     job.PTY,
		History: history,
		Stream:  stream,
		Release: unsub,
	}, nil
}

// List returns all managed jobs.
func (m *Manager) List() []ManagedJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]ManagedJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, *job)
	}
	return result
}

// IsRunning checks if any jobs are currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range m.jobs {
		if job.Status == domain.JobStatusRunning {
			return true
		}
	}
	return false
}

func (m *Manager) markStopped(job *ManagedJob) {
	job.PTY.Close()

	m.mu.Lock()
	job.Status = domain.JobStatusStopped
	m.mu.Unlock()
}

func (m *Manager) stopWithCommand(job *ManagedJob) error {
	parts := strings.Fields(job.Config.Stop)
	if len(parts) == 0 {
		return m.stopWithSignal(job)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = job.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop %s: %s: %w", job.Name, strings.TrimSpace(string(out)), err)
	}

	m.markStopped(job)
	return nil
}

func (m *Manager) stopWithSignal(job *ManagedJob) error {
	if job.Cmd.Process == nil {
		return nil
	}

	if err := job.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal %s: %w", job.Name, err)
	}

	m.markStopped(job)
	return nil
}

func (m *Manager) waitForExit(job *ManagedJob) {
	_ = job.Cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if job.Status != domain.JobStatusRunning {
		return
	}

	// Detached services (with a stop command) persist after the launcher
	// exits: the real work keeps running. Keep them marked as Running so they
	// can be found and stopped later.
	if job.Config.Stop != "" {
		return
	}

	job.Status = domain.JobStatusCrashed
}
