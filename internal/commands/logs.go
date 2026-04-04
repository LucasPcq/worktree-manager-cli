package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewLogsCmd creates the wtm logs command.
func NewLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <service>",
		Short: "Attach to a running service and stream its output",
		Long:  "Attach to a running service PTY. Press Ctrl+C to detach.",
		Args:  cobra.ExactArgs(1),
		RunE:  runLogs,
	}
}

func runLogs(_ *cobra.Command, args []string) error {
	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		// Fallback to reasonable defaults
		cols = 80
		rows = 24
	}

	client := process.NewClient(socketPath)
	conn, err := client.Attach(process.AttachParams{
		Name: args[0],
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return fmt.Errorf("attach to %s: %w", args[0], err)
	}
	defer conn.Close()

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("set terminal raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	done := make(chan struct{}, 1)

	// PTY → stdout
	go func() {
		io.Copy(os.Stdout, conn)
		done <- struct{}{}
	}()

	// stdin → PTY, detect Ctrl+C to detach
	go func() {
		buf := make([]byte, 1024)
		for {
			n, readErr := os.Stdin.Read(buf)
			if readErr != nil {
				break
			}
			for i := range n {
				if buf[i] == domain.CtrlCByte {
					done <- struct{}{}
					return
				}
			}
			if _, writeErr := conn.Write(buf[:n]); writeErr != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	return nil
}
