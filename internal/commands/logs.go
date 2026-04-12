package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
)

// newSvcLogsCmd creates the wtm svc logs subcommand.
func newSvcLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [service]",
		Short: "Stream service output",
		Long:  "Without arguments, stream all running services (multiplexed).\nWith a service name, attach to that single service PTY.\nPress Ctrl+C to detach.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runLogs,
	}
}

func runLogs(_ *cobra.Command, args []string) error {
	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	dir, _ := os.Getwd()

	if len(args) > 0 {
		return attachSingleService(socketPath, args[0], dir)
	}

	return multiplexAllServices(socketPath, dir)
}

func attachSingleService(socketPath string, name string, dir string) error {
	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		cols = 80
		rows = 24
	}

	client := process.NewClient(socketPath)
	conn, err := client.Attach(process.AttachParams{
		Name:    name,
		WorkDir: dir,
		Cols:    cols,
		Rows:    rows,
	})
	if err != nil {
		return fmt.Errorf("attach to %s: %w", name, err)
	}
	defer conn.Close()

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("set terminal raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	done := make(chan struct{}, 1)

	go func() {
		io.Copy(os.Stdout, conn)
		done <- struct{}{}
	}()

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

// serviceColors cycles through distinct colors for each service prefix.
var serviceColors = []func(string) string{
	func(s string) string { return styles.Primary.Render(s) },
	func(s string) string { return styles.Success.Render(s) },
	func(s string) string { return styles.Warning.Render(s) },
	func(s string) string { return styles.Muted.Render(s) },
}

func multiplexAllServices(socketPath string, dir string) error {
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	var running []process.ServiceInfo
	for _, svc := range resp.Services {
		if svc.WorkDir == dir && svc.Status == domain.ServiceStatusRunning {
			running = append(running, svc)
		}
	}

	if len(running) == 0 {
		fmt.Println("No running services in this worktree.")
		return nil
	}

	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		cols = 80
		rows = 24
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Attach to each service and prefix output
	for i, svc := range running {
		colorFn := serviceColors[i%len(serviceColors)]
		prefix := colorFn(fmt.Sprintf("[%s]", svc.Name))

		conn, attachErr := client.Attach(process.AttachParams{
			Name:    svc.Name,
			WorkDir: dir,
			Cols:    cols,
			Rows:    rows,
		})
		if attachErr != nil {
			output.Error(os.Stderr, fmt.Sprintf("%s: %v", svc.Name, attachErr))
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				select {
				case <-done:
					return
				default:
					fmt.Printf("%s %s\n", prefix, scanner.Text())
				}
			}
		}()
	}

	// Wait for Ctrl+C
	go func() {
		buf := make([]byte, 1)
		for {
			n, readErr := os.Stdin.Read(buf)
			if readErr != nil || n == 0 {
				break
			}
			if buf[0] == domain.CtrlCByte {
				close(done)
				return
			}
		}
	}()

	wg.Wait()
	return nil
}
