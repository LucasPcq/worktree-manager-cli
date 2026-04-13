// Package process manages long-running services with pseudo-terminals.
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

// launcherOutputBufferSize bounds how much PTY output we keep around while
// waiting for a launcher-style service (e.g. `docker compose up -d`) to exit.
// Enough for the typical Docker/compose error line without unbounded memory.
const launcherOutputBufferSize = 8 * 1024

// outputHistoryBytes is the size of the per-service rolling output buffer that
// the daemon drains from each long-running PTY. Replayed to clients on attach.
const outputHistoryBytes = 64 * 1024

// defaultPTYRows and defaultPTYCols are the fallback PTY dimensions used when a
// service is spawned before any client has attached. TUI apps read the PTY
// size at startup — a 0x0 window makes them bail to plain log mode.
const (
	defaultPTYRows = 40
	defaultPTYCols = 120
)

// ManagedService holds the state of a running service.
type ManagedService struct {
	Name    string
	Config  domain.ServiceConfig
	Cmd     *exec.Cmd
	PTY     *os.File
	Status  domain.ServiceStatus
	PID     int
	WorkDir string
	output  *outputHub // nil for launcher-style services
}

// Manager tracks and controls running services.
type Manager struct {
	services map[string]*ManagedService
	mu       sync.Mutex
}

// NewManager creates a new process manager.
func NewManager() *Manager {
	return &Manager{
		services: make(map[string]*ManagedService),
	}
}

// serviceKey returns a unique identifier for a service scoped to its worktree.
func serviceKey(name string, workDir string) string {
	return workDir + ":" + name
}

// Start launches a service in a PTY.
//
// For launcher-style services (those with a `Stop` command — e.g.
// `docker compose up -d`), Start blocks synchronously until the launcher
// process exits, so that failures like port conflicts or image pull errors
// surface to the caller instead of silently reporting "started". The lock is
// released during that wait so other operations (Stop, List, Status) remain
// responsive.
func (m *Manager) Start(svc domain.ServiceConfig, workDir string) error {
	key := serviceKey(svc.Name, workDir)

	parts := strings.Fields(svc.Cmd)
	if len(parts) == 0 {
		return fmt.Errorf("service %s has empty cmd", svc.Name)
	}

	m.mu.Lock()
	if existing, ok := m.services[key]; ok && existing.Status == domain.ServiceStatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("service %s is already running", svc.Name)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	if svc.Cwd != "" && !filepath.IsAbs(svc.Cwd) {
		cmd.Dir = filepath.Join(workDir, svc.Cwd)
	} else if svc.Cwd != "" {
		cmd.Dir = svc.Cwd
	} else {
		cmd.Dir = workDir
	}
	cmd.Env = serviceEnv()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start service %s: %w", svc.Name, err)
	}

	// Initialize the PTY to a reasonable size before the child reads it.
	// TUI frameworks like Ink (turbo, pnpm dev) decide at startup whether to
	// render based on window size — a 0x0 PTY makes them fall back to plain
	// log mode permanently. The real size is re-synced when a client attaches.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: defaultPTYRows, Cols: defaultPTYCols})

	managed := &ManagedService{
		Name:    svc.Name,
		Config:  svc,
		Cmd:     cmd,
		PTY:     ptmx,
		Status:  domain.ServiceStatusRunning,
		PID:     cmd.Process.Pid,
		WorkDir: workDir,
	}
	m.services[key] = managed
	m.mu.Unlock()

	if svc.Stop != "" {
		if err := m.waitLauncher(managed); err != nil {
			m.mu.Lock()
			delete(m.services, key)
			m.mu.Unlock()
			return err
		}
		return nil
	}

	managed.output = newOutputHub(outputHistoryBytes)
	go m.drainToHub(managed)
	go m.waitForExit(managed)
	return nil
}

// serviceEnv returns the environment to use for spawned services. The daemon
// runs with Setsid so its own TTY-related env may be missing or degraded; we
// force reasonable defaults so interactive TUIs render. Values already present
// in os.Environ() take precedence because Go's exec.Cmd honors the last
// occurrence of a given name.
func serviceEnv() []string {
	env := os.Environ()
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

func lookupEnv(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix), true
		}
	}
	return "", false
}

// waitLauncher drains PTY output into a bounded buffer and blocks until the
// launcher process exits. On non-zero exit, returns an error containing the
// captured output so the user sees what went wrong. On success, the service
// stays registered as Running (the real work is detached).
func (m *Manager) waitLauncher(svc *ManagedService) error {
	buf := newRingBuffer(launcherOutputBufferSize)
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, svc.PTY)
		close(drained)
	}()

	err := svc.Cmd.Wait()

	// Closing the PTY unblocks the drain goroutine in case Copy is still
	// reading; on normal exit it returns on its own.
	_ = svc.PTY.Close()
	<-drained

	if err != nil {
		out := cleanPTYOutput(buf.String())
		if out == "" {
			return fmt.Errorf("service %s failed: %w", svc.Name, err)
		}
		return fmt.Errorf("service %s failed:\n%s", svc.Name, out)
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
// subscribers. One service → one hub → at most one subscriber at a time
// (matches wtm's single-attach model).
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
		return nil, nil, nil, fmt.Errorf("service output closed")
	}
	if h.sub != nil {
		return nil, nil, nil, fmt.Errorf("service already has a subscriber")
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

// drainToHub copies PTY output into the service's output hub until the PTY
// closes. Long-running services need a continuous reader so the OS PTY buffer
// never fills up (which would block the service's writes before any client
// attaches).
func (m *Manager) drainToHub(svc *ManagedService) {
	_, _ = io.Copy(svc.output, svc.PTY)
	svc.output.close()
}

// Stop stops a service by name and workDir.
func (m *Manager) Stop(name string, workDir string) error {
	return m.stopByKey(serviceKey(name, workDir))
}

// StopAll stops every running service across every worktree. Intended for
// daemon shutdown; callers that want to stop only one worktree's services
// must use StopAllInWorkDir.
func (m *Manager) StopAll() error {
	return m.stopAllMatching(func(*ManagedService) bool { return true })
}

// StopAllInWorkDir stops every running service attached to the given workDir.
// Services belonging to other worktrees are left untouched.
func (m *Manager) StopAllInWorkDir(workDir string) error {
	return m.stopAllMatching(func(svc *ManagedService) bool {
		return svc.WorkDir == workDir
	})
}

func (m *Manager) stopAllMatching(keep func(*ManagedService) bool) error {
	m.mu.Lock()
	keys := make([]string, 0, len(m.services))
	for key, svc := range m.services {
		if svc.Status != domain.ServiceStatusRunning {
			continue
		}
		if !keep(svc) {
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
	svc, ok := m.services[key]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("service not found")
	}

	// Always run the stop command if configured — handles detached processes
	// like "docker compose up -d" where the launcher exits but services keep running.
	if svc.Config.Stop != "" {
		return m.stopWithCommand(svc)
	}

	if svc.Status != domain.ServiceStatusRunning {
		return nil
	}

	return m.stopWithSignal(svc)
}

// GetPTY returns the PTY file descriptor for a service (for attach).
func (m *Manager) GetPTY(name string, workDir string) (*os.File, error) {
	key := serviceKey(name, workDir)
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, ok := m.services[key]
	if !ok {
		return nil, fmt.Errorf("service %s not found", name)
	}
	if svc.Status != domain.ServiceStatusRunning {
		return nil, fmt.Errorf("service %s is not running", name)
	}
	return svc.PTY, nil
}

// AttachSession holds everything a client needs to stream a service's output:
// the PTY (for stdin forwarding and window-size ioctls), the history to
// replay, and a channel delivering subsequent output. The caller must invoke
// Release when done.
type AttachSession struct {
	PTY     *os.File
	History []byte
	Stream  <-chan []byte
	Release func()
}

// Attach subscribes to a service's output hub and returns a session the daemon
// can use to stream history + live output to a client while still forwarding
// the client's stdin back into the PTY.
func (m *Manager) Attach(name string, workDir string) (*AttachSession, error) {
	key := serviceKey(name, workDir)
	m.mu.Lock()
	svc, ok := m.services[key]
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("service %s not found", name)
	}
	if svc.Status != domain.ServiceStatusRunning {
		return nil, fmt.Errorf("service %s is not running", name)
	}
	if svc.output == nil {
		return nil, fmt.Errorf("service %s has no attachable output (launcher-style)", name)
	}

	history, stream, unsub, err := svc.output.Subscribe()
	if err != nil {
		return nil, err
	}
	return &AttachSession{
		PTY:     svc.PTY,
		History: history,
		Stream:  stream,
		Release: unsub,
	}, nil
}

// List returns all managed services.
func (m *Manager) List() []ManagedService {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]ManagedService, 0, len(m.services))
	for _, svc := range m.services {
		result = append(result, *svc)
	}
	return result
}

// IsRunning checks if any services are currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, svc := range m.services {
		if svc.Status == domain.ServiceStatusRunning {
			return true
		}
	}
	return false
}

func (m *Manager) markStopped(svc *ManagedService) {
	svc.PTY.Close()

	m.mu.Lock()
	svc.Status = domain.ServiceStatusStopped
	m.mu.Unlock()
}

func (m *Manager) stopWithCommand(svc *ManagedService) error {
	parts := strings.Fields(svc.Config.Stop)
	if len(parts) == 0 {
		return m.stopWithSignal(svc)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = svc.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop %s: %s: %w", svc.Name, strings.TrimSpace(string(out)), err)
	}

	m.markStopped(svc)
	return nil
}

func (m *Manager) stopWithSignal(svc *ManagedService) error {
	if svc.Cmd.Process == nil {
		return nil
	}

	if err := svc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal %s: %w", svc.Name, err)
	}

	m.markStopped(svc)
	return nil
}

func (m *Manager) waitForExit(svc *ManagedService) {
	_ = svc.Cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if svc.Status != domain.ServiceStatusRunning {
		return
	}

	// Services with a stop command (e.g. "docker compose up -d") are detached:
	// the launcher process exits but the service keeps running.
	// Keep them marked as Running so they can be found and stopped later.
	if svc.Config.Stop != "" {
		return
	}

	svc.Status = domain.ServiceStatusCrashed
}
