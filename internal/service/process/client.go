package process

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

var daemonStartTimeout = time.Duration(domain.DaemonStartTimeoutSeconds) * time.Second

// Client communicates with the daemon over a Unix socket.
type Client struct {
	socketPath string
}

// NewClient creates a client for the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// Send sends a request to the daemon and returns the first terminal response
// (StatusOK, StatusDone, or StatusError). Intermediate StatusOutput chunks
// are discarded — use SendStream when the caller wants to forward them.
func (c *Client) Send(req Request) (Response, error) {
	return c.SendStream(req, nil)
}

// SendStream sends the request and iterates over every response. `onOutput`
// is invoked with each StatusOutput chunk's Data (useful for streaming task
// stdout/stderr to the user). Returns the first terminal response
// (StatusOK, StatusDone, or StatusError) received.
func (c *Client) SendStream(req Request, onOutput func([]byte)) (Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}

	for {
		var resp Response
		if err := decoder.Decode(&resp); err != nil {
			return Response{}, fmt.Errorf("read response: %w", err)
		}
		if resp.Status == StatusOutput {
			if onOutput != nil && len(resp.Data) > 0 {
				onOutput(resp.Data)
			}
			continue
		}
		return resp, nil
	}
}

// AttachParams holds inputs for attaching to a service.
type AttachParams struct {
	Name    string
	WorkDir string
	Cols    int
	Rows    int
}

// Attach sends an attach request and returns the raw connection on success.
// The caller is responsible for closing the connection.
func (c *Client) Attach(params AttachParams) (net.Conn, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{
		Action:  ActionAttach,
		Name:    params.Name,
		WorkDir: params.WorkDir,
		Cols:    params.Cols,
		Rows:    params.Rows,
	}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send attach request: %w", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read attach response: %w", err)
	}

	if resp.Status != StatusOK {
		conn.Close()
		return nil, fmt.Errorf("attach failed: %s", resp.Message)
	}

	return conn, nil
}

// Resize asks the daemon to size a job's PTY to the pane rendering it. It
// dials its own connection rather than reusing the one Attach returned, which
// is a raw byte stream feeding the job's stdin from the moment it is accepted.
func (c *Client) Resize(params ResizeParams) error {
	resp, err := c.Send(Request{
		Action:  ActionResize,
		Name:    params.Name,
		WorkDir: params.WorkDir,
		Cols:    params.Cols,
		Rows:    params.Rows,
	})
	if err != nil {
		return err
	}
	if resp.Status != StatusOK {
		return fmt.Errorf("resize failed: %s", resp.Message)
	}
	return nil
}

// IsDaemonRunning checks if a daemon is listening on the socket.
func IsDaemonRunning(socketPath string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// EnsureDaemon checks if the daemon is running; if not, starts it and waits.
func EnsureDaemon(socketPath string) error {
	if IsDaemonRunning(socketPath) {
		return nil
	}

	// Remove stale socket if present
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	if err := StartDaemon(socketPath); err != nil {
		return err
	}

	// Poll until the daemon is ready
	deadline := time.Now().Add(daemonStartTimeout)
	for time.Now().Before(deadline) {
		if IsDaemonRunning(socketPath) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not start within %v", daemonStartTimeout)
}

// StartDaemon forks "wtm daemon" as a detached background process.
func StartDaemon(socketPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	cmd := exec.Command(exePath, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Detach — don't wait for the daemon process
	return nil
}
