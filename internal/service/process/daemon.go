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

	"github.com/LucasPcq/wtm/internal/domain"
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
	if req.Service == nil {
		encoder.Encode(Response{Status: StatusError, Message: "service config required"})
		return
	}

	if err := d.manager.Start(*req.Service, req.WorkDir); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("service %s started", req.Service.Name)})
}

func (d *daemonServer) handleStop(encoder *json.Encoder, req Request) {
	if err := d.manager.Stop(req.Name, req.WorkDir); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("service %s stopped", req.Name)})
}

func (d *daemonServer) handleStopAll(encoder *json.Encoder, req Request) {
	// Snapshot the services that are about to be stopped so the client can
	// report which ones (or say "none running" when the list is empty).
	var stopped []ServiceInfo
	for _, svc := range d.manager.List() {
		if svc.Status != domain.ServiceStatusRunning {
			continue
		}
		if req.WorkDir != "" && svc.WorkDir != req.WorkDir {
			continue
		}
		stopped = append(stopped, ServiceInfo{
			Name:    svc.Name,
			WorkDir: svc.WorkDir,
			Status:  svc.Status,
			PID:     svc.PID,
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

	encoder.Encode(Response{Status: StatusOK, Services: stopped})
}

func (d *daemonServer) handleList(encoder *json.Encoder, req Request) {
	services := d.manager.List()
	infos := make([]ServiceInfo, 0, len(services))
	for _, svc := range services {
		infos = append(infos, ServiceInfo{
			Name:    svc.Name,
			WorkDir: svc.WorkDir,
			Status: svc.Status,
			PID:    svc.PID,
		})
	}

	encoder.Encode(Response{Status: StatusOK, Services: infos})
}

func (d *daemonServer) handleAttach(conn net.Conn, encoder *json.Encoder, req Request) {
	ptmx, err := d.manager.GetPTY(req.Name, req.WorkDir)
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	// Set initial window size if provided
	if req.Cols > 0 && req.Rows > 0 {
		syscall.Syscall(
			syscall.SYS_IOCTL,
			ptmx.Fd(),
			syscall.TIOCSWINSZ,
			uintptr(unsafeWinsize(req.Cols, req.Rows)),
		)
	}

	// Send OK before switching to raw mode
	encoder.Encode(Response{Status: StatusOK, Message: "attached"})

	// Bidirectional copy: socket ↔ PTY
	done := make(chan struct{}, 1)

	go func() {
		io.Copy(conn, ptmx)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(ptmx, conn)
		done <- struct{}{}
	}()

	// Wait for either side to close
	<-done
}

// SocketPath returns the default daemon socket path.
func SocketPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, domain.GlobalConfigDir, domain.DaemonSocketName)
}
