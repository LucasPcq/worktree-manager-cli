package process

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
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
	return c.SendStreamContext(context.Background(), req, onOutput)
}

// SendStreamContext is SendStream, given up on when ctx is done: the connection
// is closed, which unblocks the read, and the call returns the context's error.
// The daemon is not told anything — the job it is running is untouched, and only
// this conversation about it ends.
func (c *Client) SendStreamContext(ctx context.Context, req Request, onOutput func([]byte)) (Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	defer context.AfterFunc(ctx, func() { conn.Close() })()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}

	for {
		var resp Response
		if err := decoder.Decode(&resp); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Response{}, ctxErr
			}
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

	held, err := io.ReadAll(decoder.Buffered())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read attach response: %w", err)
	}

	return prefixedConn{
		Conn:   conn,
		reader: io.MultiReader(bytes.NewReader(afterResponseDelimiter(held)), conn),
	}, nil
}

// afterResponseDelimiter drops the newline the encoder writes to close the
// attach response, which the decoder leaves in its buffer. Everything past it
// is the job's own output.
func afterResponseDelimiter(held []byte) []byte {
	held = bytes.TrimPrefix(held, []byte("\r"))
	return bytes.TrimPrefix(held, []byte("\n"))
}

// prefixedConn replays what the decoder read past the attach response before
// the rest of the connection. The daemon writes the job's buffered history
// right behind that response, so a single read commonly carries both, and
// whatever the decoder kept would otherwise be dropped with it.
type prefixedConn struct {
	net.Conn
	reader io.Reader
}

func (c prefixedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

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
func EnsureDaemon(params DaemonParams) error {
	if IsDaemonRunning(params.SocketPath) {
		return nil
	}

	// Remove stale socket if present
	if _, err := os.Stat(params.SocketPath); err == nil {
		os.Remove(params.SocketPath)
	}

	if err := StartDaemon(params); err != nil {
		return err
	}

	// Poll until the daemon is ready
	deadline := time.Now().Add(daemonStartTimeout)
	for time.Now().Before(deadline) {
		if IsDaemonRunning(params.SocketPath) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not start within %v", daemonStartTimeout)
}

// StartDaemon forks "wtm daemon" as a detached background process. The proxy
// port travels on the command line because only a client can read the user's
// global config — the daemon is global and outlives any one of them.
func StartDaemon(params DaemonParams) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	args := []string{domain.CmdDaemon}
	if params.ProxyPort > 0 {
		args = append(args, "--"+domain.FlagProxyPort, strconv.Itoa(params.ProxyPort))
	}
	cmd := exec.Command(exePath, args...)
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
