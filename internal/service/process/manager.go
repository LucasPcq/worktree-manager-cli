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

// ManagedService holds the state of a running service.
type ManagedService struct {
	Name    string
	Config  domain.ServiceConfig
	Cmd     *exec.Cmd
	PTY     *os.File
	Status  domain.ServiceStatus
	PID     int
	WorkDir string
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

	ptmx, err := pty.Start(cmd)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start service %s: %w", svc.Name, err)
	}

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

	go m.waitForExit(managed)
	return nil
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
