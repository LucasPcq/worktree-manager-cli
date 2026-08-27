package process

import (
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// routeRecorder stands in for the proxy registry: it records what the manager
// publishes and withdraws, under a lock because the reaping goroutine writes too.
type routeRecorder struct {
	mu      sync.Mutex
	added   []domain.ProxyRoute
	removed []string
}

func (r *routeRecorder) Add(route domain.ProxyRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.added = append(r.added, route)
}

func (r *routeRecorder) Remove(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, host)
}

func (r *routeRecorder) snapshot() ([]domain.ProxyRoute, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.ProxyRoute(nil), r.added...), append([]string(nil), r.removed...)
}

func publishedJob() domain.JobConfig {
	return domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "sleep 30",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
}

func TestManagerRoutesPublishesAStartedJob(t *testing.T) {
	routes := &routeRecorder{}
	m := NewManagerWithRoutes(routes)
	dir := t.TempDir()

	err := m.Start(StartParams{
		Job:       publishedJob(),
		WorkDir:   dir,
		Env:       map[string]string{domain.EnvPortOffset: "10", domain.EnvWorktree: "feat", domain.EnvProject: "myapp"},
		RouteHost: "web.feat.myapp.localhost",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	added, _ := routes.snapshot()
	if len(added) != 1 {
		t.Fatalf("added = %v, want exactly one route", added)
	}
	// The target carries the worktree's offset: the route has to point at the
	// port the job actually bound, not at the one it declared. It names the host
	// rather than 127.0.0.1 so a job listening on ::1 only is still reachable.
	if added[0].Target != "localhost:3010" {
		t.Errorf("Target = %q, want localhost:3010", added[0].Target)
	}
	if added[0].Host != "web.feat.myapp.localhost" || added[0].Worktree != "feat" || added[0].Project != "myapp" {
		t.Errorf("route = %+v, want the worktree it belongs to named", added[0])
	}
}

func TestManagerRoutesWithdrawsAStoppedJob(t *testing.T) {
	routes := &routeRecorder{}
	m := NewManagerWithRoutes(routes)
	dir := t.TempDir()

	if err := m.Start(StartParams{
		Job:       publishedJob(),
		WorkDir:   dir,
		Env:       map[string]string{domain.EnvPortOffset: "10"},
		RouteHost: "web.feat.myapp.localhost",
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := m.Stop("web", dir); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Stopping and reaping both withdraw, and Remove is a map delete: what the
	// test pins is that the route is gone and that nothing else was touched.
	_, removed := routes.snapshot()
	if len(removed) == 0 {
		t.Fatal("stopping a published job must withdraw its route")
	}
	for _, host := range removed {
		if host != "web.feat.myapp.localhost" {
			t.Errorf("removed %q, want only the host the job was published under", host)
		}
	}
}

func TestManagerRoutesIgnoresAJobThatPublishesNothing(t *testing.T) {
	routes := &routeRecorder{}
	m := NewManagerWithRoutes(routes)
	dir := t.TempDir()

	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "sleep 30", Ports: map[string]int{"PG_PORT": 5432}}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	added, _ := routes.snapshot()
	if len(added) != 0 {
		t.Errorf("added = %v, want nothing: the job declares no url", added)
	}
}
