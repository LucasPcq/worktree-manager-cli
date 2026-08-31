package run

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// attachSettle is how long the fake daemon holds a job's output back after
// accepting an attach; see serve.
// fakeDaemon answers on the socket process.SocketPath resolves to, with the
// config dir redirected under a short temp home: a command under test dials it
// exactly as it dials the real daemon, and never forks one.
type fakeDaemon struct {
	// Answers maps a job name to the responses ActionStart replies with, in
	// order. A job with no script is started without a word.
	Answers map[string][]process.Response
	// Jobs is what ActionList reports.
	Jobs []domain.JobInfo
	// Streams maps a job name to the raw bytes ActionAttach writes on the
	// accepted connection before hanging up.
	Streams map[string][]byte

	mu       sync.Mutex
	requests []process.Request
}

// shortHome points the config directory — and so the daemon socket under it —
// at /tmp rather than t.TempDir(), whose name overruns sun_path on macOS (104
// bytes) and fails the bind with EINVAL.
func shortHome(t *testing.T) {
	t.Helper()

	// Idempotent: a test that both starts a daemon and sets up a project calls
	// this twice, and a second home would move the socket out from under the
	// daemon already listening on the first.
	if strings.HasPrefix(os.Getenv("HOME"), "/tmp/wtmhome") {
		return
	}

	home, err := os.MkdirTemp("/tmp", "wtmhome")
	if err != nil {
		t.Fatalf("temp home: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// startFakeDaemon binds the daemon socket for the length of the test.
func startFakeDaemon(t *testing.T, daemon *fakeDaemon) *fakeDaemon {
	t.Helper()

	shortHome(t)

	socket := process.SocketPath()
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatalf("socket dir: %v", err)
	}

	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen on %s: %v", socket, err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go daemon.serve(conn)
		}
	}()

	return daemon
}

func (d *fakeDaemon) serve(conn net.Conn) {
	defer conn.Close()

	var req process.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	d.mu.Lock()
	d.requests = append(d.requests, req)
	d.mu.Unlock()

	encoder := json.NewEncoder(conn)
	if req.Action == process.ActionList {
		_ = encoder.Encode(process.Response{Status: process.StatusOK, Jobs: d.Jobs})
		return
	}
	if req.Action == process.ActionAttach {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		_, _ = conn.Write(d.Streams[req.Name])
		return
	}
	if req.Action != process.ActionStart || req.Job == nil {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		return
	}

	scripted, ok := d.Answers[req.Job.Name]
	if !ok {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		return
	}
	for _, resp := range scripted {
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

// startedJobs names the jobs the commands asked the daemon to start, in order.
func (d *fakeDaemon) startedJobs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	names := make([]string, 0, len(d.requests))
	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil {
			names = append(names, req.Job.Name)
		}
	}
	return names
}
