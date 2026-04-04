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

// Send sends a request to the daemon and returns the response.
func (c *Client) Send(req Request) (Response, error) {
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

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	return resp, nil
}

// AttachParams holds inputs for attaching to a service.
type AttachParams struct {
	Name string
	Cols int
	Rows int
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
		Action: ActionAttach,
		Name:   params.Name,
		Cols:   params.Cols,
		Rows:   params.Rows,
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
