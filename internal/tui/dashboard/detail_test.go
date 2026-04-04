package dashboard

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func TestFilterServicesByWorkDir(t *testing.T) {
	services := []process.ServiceInfo{
		{Name: "api", Status: domain.ServiceStatusRunning, PID: 100, WorkDir: "/projects/alpha"},
		{Name: "web", Status: domain.ServiceStatusRunning, PID: 200, WorkDir: "/projects/beta"},
		{Name: "worker", Status: domain.ServiceStatusStopped, PID: 300, WorkDir: "/projects/alpha"},
	}

	result := filterServicesByWorkDir(services, "/projects/alpha")
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result))
	}
}

func TestFilterServicesByWorkDirEmpty(t *testing.T) {
	result := filterServicesByWorkDir([]process.ServiceInfo{}, "/projects/alpha")
	if len(result) != 0 {
		t.Fatalf("expected 0 matches for empty input, got %d", len(result))
	}
}

func TestFilterServicesByWorkDirNoMatch(t *testing.T) {
	services := []process.ServiceInfo{
		{Name: "api", Status: domain.ServiceStatusRunning, PID: 100, WorkDir: "/projects/alpha"},
		{Name: "web", Status: domain.ServiceStatusRunning, PID: 200, WorkDir: "/projects/beta"},
	}

	result := filterServicesByWorkDir(services, "/projects/gamma")
	if len(result) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(result))
	}
}
