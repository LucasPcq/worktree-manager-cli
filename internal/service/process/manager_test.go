package process

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestStartService(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	cfg := domain.ServiceConfig{Name: "test", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, workDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	services := mgr.List()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Status != domain.ServiceStatusRunning {
		t.Errorf("expected status %s, got %s", domain.ServiceStatusRunning, services[0].Status)
	}
	if services[0].PID <= 0 {
		t.Errorf("expected PID > 0, got %d", services[0].PID)
	}
}

func TestStopService(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	cfg := domain.ServiceConfig{Name: "stopper", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, workDir); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	if err := mgr.Stop("stopper", workDir); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	services := mgr.List()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Status != domain.ServiceStatusStopped {
		t.Errorf("expected status %s, got %s", domain.ServiceStatusStopped, services[0].Status)
	}
}

func TestStopAll(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	cfgA := domain.ServiceConfig{Name: "svc-a", Cmd: "sleep 60"}
	cfgB := domain.ServiceConfig{Name: "svc-b", Cmd: "sleep 60"}

	if err := mgr.Start(cfgA, workDir); err != nil {
		t.Fatalf("unexpected start error for svc-a: %v", err)
	}
	if err := mgr.Start(cfgB, workDir); err != nil {
		t.Fatalf("unexpected start error for svc-b: %v", err)
	}

	if err := mgr.StopAll(); err != nil {
		t.Fatalf("unexpected StopAll error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	for _, svc := range mgr.List() {
		if svc.Status != domain.ServiceStatusStopped {
			t.Errorf("service %s: expected status %s, got %s", svc.Name, domain.ServiceStatusStopped, svc.Status)
		}
	}
}

func TestStartDuplicate(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	cfg := domain.ServiceConfig{Name: "dup", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, workDir); err != nil {
		t.Fatalf("unexpected first start error: %v", err)
	}

	err := mgr.Start(cfg, workDir)
	if err == nil {
		t.Fatal("expected error on duplicate start, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected error containing 'already running', got: %v", err)
	}
}

func TestStartSameNameDifferentWorkDir(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	cfg := domain.ServiceConfig{Name: "api", Cmd: "sleep 60"}
	workDirA := t.TempDir()
	workDirB := t.TempDir()

	if err := mgr.Start(cfg, workDirA); err != nil {
		t.Fatalf("unexpected error starting in A: %v", err)
	}
	if err := mgr.Start(cfg, workDirB); err != nil {
		t.Fatalf("unexpected error starting in B: %v", err)
	}

	services := mgr.List()
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
}

func TestGetPTY(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	cfg := domain.ServiceConfig{Name: "pty-svc", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, workDir); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	ptmx, err := mgr.GetPTY("pty-svc", workDir)
	if err != nil {
		t.Fatalf("unexpected GetPTY error: %v", err)
	}
	if ptmx == nil {
		t.Error("expected non-nil PTY file")
	}

	_, err = mgr.GetPTY("nonexistent", workDir)
	if err == nil {
		t.Error("expected error for nonexistent service, got nil")
	}
}

func TestStartLauncher_SuccessExit0(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	// Launcher-style: has a Stop command. Simulates `docker compose up -d`
	// that exits 0 after detaching.
	cfg := domain.ServiceConfig{
		Name: "launcher-ok",
		Cmd:  "true",
		Stop: "true",
	}
	if err := mgr.Start(cfg, workDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	services := mgr.List()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Status != domain.ServiceStatusRunning {
		t.Errorf("launcher exit 0 should keep status Running, got %s", services[0].Status)
	}
}

func TestStopAllInWorkDir_LeavesOtherWorktreesAlone(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	cfgA := domain.ServiceConfig{Name: "api", Cmd: "sleep 60"}
	cfgB := domain.ServiceConfig{Name: "api", Cmd: "sleep 60"}
	workDirA := t.TempDir()
	workDirB := t.TempDir()

	if err := mgr.Start(cfgA, workDirA); err != nil {
		t.Fatalf("start A: %v", err)
	}
	if err := mgr.Start(cfgB, workDirB); err != nil {
		t.Fatalf("start B: %v", err)
	}

	if err := mgr.StopAllInWorkDir(workDirA); err != nil {
		t.Fatalf("StopAllInWorkDir: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	var stoppedA, runningB bool
	for _, svc := range mgr.List() {
		if svc.WorkDir == workDirA && svc.Status == domain.ServiceStatusStopped {
			stoppedA = true
		}
		if svc.WorkDir == workDirB && svc.Status == domain.ServiceStatusRunning {
			runningB = true
		}
	}
	if !stoppedA {
		t.Error("expected service in workDirA to be stopped")
	}
	if !runningB {
		t.Error("expected service in workDirB to remain running")
	}
}

func TestStartLauncher_FailureSurfacesError(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	workDir := t.TempDir()
	// `false` exits 1 with no output — simulates a launcher that fails
	// (bad compose file, port conflict, etc). Real Docker output would be
	// longer; the ring buffer drains whatever shows up.
	cfg := domain.ServiceConfig{
		Name: "launcher-fail",
		Cmd:  "false",
		Stop: "true",
	}
	err := mgr.Start(cfg, workDir)
	if err == nil {
		t.Fatal("expected error when launcher exits non-zero, got nil")
	}
	if !strings.Contains(err.Error(), "launcher-fail") {
		t.Errorf("error should mention service name, got: %v", err)
	}

	// Failed launcher must not be registered — it never ran successfully.
	if len(mgr.List()) != 0 {
		t.Errorf("failed launcher should not be registered, got %d entries", len(mgr.List()))
	}
}
