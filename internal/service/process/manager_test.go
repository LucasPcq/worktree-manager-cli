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

	cfg := domain.ServiceConfig{Name: "test", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, t.TempDir()); err != nil {
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

	cfg := domain.ServiceConfig{Name: "stopper", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, t.TempDir()); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	if err := mgr.Stop("stopper"); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}

	// Allow the waitForExit goroutine to complete
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

	cfgA := domain.ServiceConfig{Name: "svc-a", Cmd: "sleep 60"}
	cfgB := domain.ServiceConfig{Name: "svc-b", Cmd: "sleep 60"}

	if err := mgr.Start(cfgA, t.TempDir()); err != nil {
		t.Fatalf("unexpected start error for svc-a: %v", err)
	}
	if err := mgr.Start(cfgB, t.TempDir()); err != nil {
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

	cfg := domain.ServiceConfig{Name: "dup", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, t.TempDir()); err != nil {
		t.Fatalf("unexpected first start error: %v", err)
	}

	err := mgr.Start(cfg, t.TempDir())
	if err == nil {
		t.Fatal("expected error on duplicate start, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected error containing 'already running', got: %v", err)
	}
}

func TestGetPTY(t *testing.T) {
	mgr := NewManager()
	defer mgr.StopAll()

	cfg := domain.ServiceConfig{Name: "pty-svc", Cmd: "sleep 60"}
	if err := mgr.Start(cfg, t.TempDir()); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	ptmx, err := mgr.GetPTY("pty-svc")
	if err != nil {
		t.Fatalf("unexpected GetPTY error: %v", err)
	}
	if ptmx == nil {
		t.Error("expected non-nil PTY file")
	}

	_, err = mgr.GetPTY("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent service, got nil")
	}
}
