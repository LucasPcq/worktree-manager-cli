package process

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Attach connects stdin/stdout to a service PTY for interactive terminal access.
// Returns when the user presses Ctrl+C or the PTY closes.
func Attach(ptmx *os.File) error {
	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("set raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Handle window resize
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)

	go func() {
		for range resizeCh {
			syncTerminalSize(ptmx)
		}
	}()

	// Sync initial size
	syncTerminalSize(ptmx)

	// Handle Ctrl+C gracefully — detach instead of killing the service
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	doneCh := make(chan struct{})

	// Copy PTY output to stdout
	go func() {
		io.Copy(os.Stdout, ptmx)
		close(doneCh)
	}()

	// Copy stdin to PTY (stop on Ctrl+C)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			// Detect Ctrl+C — detach instead of forwarding
			for i := 0; i < n; i++ {
				if buf[i] == domain.CtrlCByte {
					close(doneCh)
					return
				}
			}
			ptmx.Write(buf[:n])
		}
	}()

	// Wait for detach or PTY close
	select {
	case <-doneCh:
	case <-sigCh:
	}

	return nil
}

func syncTerminalSize(ptmx *os.File) {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	_ = setWinsize(winsizeParams{File: ptmx, Cols: width, Rows: height})
}
