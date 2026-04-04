// Package process manages long-running services with pseudo-terminals.
package process

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/internal/domain"
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

// Start launches a service in a PTY.
func (m *Manager) Start(svc domain.ServiceConfig, workDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.services[svc.Name]; ok && existing.Status == domain.ServiceStatusRunning {
		return fmt.Errorf("service %s is already running", svc.Name)
	}

	parts := strings.Fields(svc.Cmd)
	if len(parts) == 0 {
		return fmt.Errorf("service %s has empty cmd", svc.Name)
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	if svc.Cwd != "" {
		cmd.Dir = svc.Cwd
	} else {
		cmd.Dir = workDir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
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

	m.services[svc.Name] = managed

	// Monitor process exit in background
	go m.waitForExit(managed)

	return nil
}

// Stop stops a service. Uses the stop command if defined, otherwise sends SIGTERM.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	svc, ok := m.services[name]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if svc.Status != domain.ServiceStatusRunning {
		return nil
	}

	if svc.Config.Stop != "" {
		return m.stopWithCommand(svc)
	}

	return m.stopWithSignal(svc)
}

// StopAll stops all running services.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	names := make([]string, 0, len(m.services))
	for name, svc := range m.services {
		if svc.Status == domain.ServiceStatusRunning {
			names = append(names, name)
		}
	}
	m.mu.Unlock()

	var firstErr error
	for _, name := range names {
		if err := m.Stop(name); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetPTY returns the PTY file descriptor for a service (for attach).
func (m *Manager) GetPTY(name string) (*os.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, ok := m.services[name]
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
	cmd.Dir = svc.Cmd.Dir
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

	if svc.Status == domain.ServiceStatusRunning {
		svc.Status = domain.ServiceStatusCrashed
	}
}
