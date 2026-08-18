package process

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

var daemonIdleTimeout = time.Duration(domain.DaemonIdleTimeoutSeconds) * time.Second

// DaemonParams holds inputs for starting the daemon.
type DaemonParams struct {
	SocketPath string
}

type daemonServer struct {
	manager    *Manager
	listener   net.Listener
	socketPath string
	clients    sync.WaitGroup
	shutdown   chan struct{}
}

// RunDaemon starts the daemon, listens on the Unix socket, and blocks until shutdown.
func RunDaemon(params DaemonParams) error {
	if err := SupportedOnPlatform(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(params.SocketPath), 0o755); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket
	os.Remove(params.SocketPath)

	listener, err := net.Listen("unix", params.SocketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	d := &daemonServer{
		manager:    NewManager(),
		listener:   listener,
		socketPath: params.SocketPath,
		shutdown:   make(chan struct{}),
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		d.stop()
	}()

	// Idle timer for auto-exit
	go d.idleWatcher()

	// Accept loop
	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			select {
			case <-d.shutdown:
				return nil
			default:
				continue
			}
		}
		d.clients.Add(1)
		go func() {
			defer d.clients.Done()
			d.handleConnection(conn)
		}()
	}
}

func (d *daemonServer) stop() {
	close(d.shutdown)
	d.listener.Close()
	d.manager.StopAll()
	os.Remove(d.socketPath)
	d.clients.Wait()
}

func (d *daemonServer) idleWatcher() {
	ticker := time.NewTicker(daemonIdleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdown:
			return
		case <-ticker.C:
			if !d.manager.IsRunning() {
				d.stop()
				return
			}
		}
	}
}

func (d *daemonServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	switch req.Action {
	case ActionStart:
		d.handleStart(encoder, req)
	case ActionStop:
		d.handleStop(encoder, req)
	case ActionStopAll:
		d.handleStopAll(encoder, req)
	case ActionList:
		d.handleList(encoder, req)
	case ActionAttach:
		d.handleAttach(conn, encoder, req)
	default:
		encoder.Encode(Response{Status: StatusError, Message: fmt.Sprintf("unknown action: %s", req.Action)})
	}
}

func (d *daemonServer) handleStart(encoder *json.Encoder, req Request) {
	if req.Job == nil {
		encoder.Encode(Response{Status: StatusError, Message: "job config required"})
		return
	}

	if req.Job.Kind == domain.JobKindTask {
		// Tasks block until the command exits and stream their output back over
		// the socket as StatusOutput chunks, so the CLI can render it live
		// (`run up` / `run start`). Start blocks until every chunk has been
		// flushed, so the terminal response below never races the stream.
		err := d.manager.Start(*req.Job, req.WorkDir, responseStreamWriter{encoder: encoder})
		if err != nil {
			code := exitCodeOf(err)
			encoder.Encode(Response{Status: StatusError, Message: err.Error(), ExitCode: &code})
			return
		}
		zero := 0
		encoder.Encode(Response{Status: StatusDone, Message: fmt.Sprintf("task %s done", req.Job.Name), ExitCode: &zero})
		return
	}

	// Detached launchers (e.g. docker compose up -d) run like a task for their
	// lifetime — they stream their startup output and then exit — but stay
	// registered as Running afterwards. Stream that output live so `run up`
	// shows the launcher's lines as they happen, then send the terminal "started".
	if rules.IsDetached(*req.Job) {
		if err := d.manager.Start(*req.Job, req.WorkDir, responseStreamWriter{encoder: encoder}); err != nil {
			encoder.Encode(Response{Status: StatusError, Message: err.Error()})
			return
		}
		encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s started", req.Job.Name)})
		return
	}

	if err := d.manager.Start(*req.Job, req.WorkDir, nil); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s started", req.Job.Name)})
}

func (d *daemonServer) handleStop(encoder *json.Encoder, req Request) {
	if err := d.manager.Stop(req.Name, req.WorkDir); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s stopped", req.Name)})
}

func (d *daemonServer) handleStopAll(encoder *json.Encoder, req Request) {
	// Snapshot the jobs that are about to be stopped so the client can report
	// which ones (or say "none running" when the list is empty).
	var stopped []domain.JobInfo
	for _, job := range d.manager.List() {
		if job.Status != domain.JobStatusRunning {
			continue
		}
		if req.WorkDir != "" && job.WorkDir != req.WorkDir {
			continue
		}
		stopped = append(stopped, domain.JobInfo{
			Name:    job.Name,
			Kind:    job.Config.Kind,
			WorkDir: job.WorkDir,
			Status:  job.Status,
			PID:     job.PID,
		})
	}

	var err error
	if req.WorkDir != "" {
		err = d.manager.StopAllInWorkDir(req.WorkDir)
	} else {
		err = d.manager.StopAll()
	}
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Jobs: stopped})
}

func (d *daemonServer) handleList(encoder *json.Encoder, req Request) {
	jobs := d.manager.List()
	infos := make([]domain.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		infos = append(infos, domain.JobInfo{
			Name:    job.Name,
			Kind:    job.Config.Kind,
			WorkDir: job.WorkDir,
			Status:  job.Status,
			PID:     job.PID,
		})
	}

	encoder.Encode(Response{Status: StatusOK, Jobs: infos})
}

func (d *daemonServer) handleAttach(conn net.Conn, encoder *json.Encoder, req Request) {
	session, err := d.manager.Attach(req.Name, req.WorkDir)
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}
	defer session.Release()

	// Set initial window size if provided
	if req.Cols > 0 && req.Rows > 0 {
		_ = pty.Setsize(session.PTY, &pty.Winsize{Rows: uint16(req.Rows), Cols: uint16(req.Cols)})
	}

	// Send OK before switching to raw mode
	encoder.Encode(Response{Status: StatusOK, Message: "attached"})

	// Replay buffered history so the client sees the current TUI state
	// (progress lines, running frame, etc.) before live output resumes.
	if len(session.History) > 0 {
		conn.Write(session.History)
	}

	done := make(chan struct{}, 2)

	// Live PTY output → client. The PTY itself is drained by the hub goroutine
	// elsewhere; we just relay what the hub delivers to this subscriber.
	go func() {
		for data := range session.Stream {
			if _, err := conn.Write(data); err != nil {
				done <- struct{}{}
				return
			}
		}
		done <- struct{}{}
	}()

	// Client stdin → PTY. Direct write path; only one subscriber at a time
	// means there's no write contention.
	go func() {
		io.Copy(session.PTY, conn)
		done <- struct{}{}
	}()

	<-done
}

// responseStreamWriter adapts a job's output stream onto the daemon
// connection: each write is encoded as a StatusOutput response so the client
// can forward task output to the user in real time. It is only ever written to
// by the task's single streaming goroutine, so the encoder sees no concurrent
// use while the task runs.
type responseStreamWriter struct {
	encoder *json.Encoder
}

func (w responseStreamWriter) Write(p []byte) (int, error) {
	if err := w.encoder.Encode(Response{Status: StatusOutput, Data: p}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// SocketPath returns the default daemon socket path.
func SocketPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, domain.GlobalConfigDir, domain.DaemonSocketName)
}
