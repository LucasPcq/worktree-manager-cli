package run

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

// newLogsCmd creates the wtm run logs subcommand.
func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdLogs + " [job]",
		Short: "Attach to a job's output",
		Long:  "Without arguments, stream all running jobs (multiplexed).\nWith a job name, attach to that single job's PTY.\nPress Ctrl+C to detach.",
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
		return attachSingleJob(socketPath, args[0], dir)
	}

	return multiplexAllJobs(socketPath, dir)
}

func attachSingleJob(socketPath string, name string, dir string) error {
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
	defer process.ResetTerminalState(os.Stdout)

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

// jobColors cycles through distinct colors for each job's log prefix.
var jobColors = []func(string) string{
	func(s string) string { return styles.Primary.Render(s) },
	func(s string) string { return styles.Success.Render(s) },
	func(s string) string { return styles.Warning.Render(s) },
	func(s string) string { return styles.Muted.Render(s) },
}

func multiplexAllJobs(socketPath string, dir string) error {
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	var running []process.JobInfo
	for _, job := range resp.Jobs {
		if job.WorkDir == dir && job.Status == domain.JobStatusRunning {
			running = append(running, job)
		}
	}

	if len(running) == 0 {
		output.Blank(os.Stdout)
		output.Message(os.Stdout, "No running jobs in this worktree.")
		output.Blank(os.Stdout)
		return nil
	}

	return multiplexJobs(socketPath, dir, running)
}

// multiplexJobs attaches to every job in `running` and prints their output as
// color-prefixed lines on a single stream. Ctrl+C detaches from all of them
// without stopping the jobs. Used both by `run logs` (no args) and by
// `run up` once a profile's services are live.
func multiplexJobs(socketPath string, dir string, running []process.JobInfo) error {
	client := process.NewClient(socketPath)

	defer process.ResetTerminalState(os.Stdout)

	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		cols = 80
		rows = 24
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i, job := range running {
		colorFn := jobColors[i%len(jobColors)]
		prefix := colorFn(fmt.Sprintf("[%s]", job.Name))

		conn, attachErr := client.Attach(process.AttachParams{
			Name:    job.Name,
			WorkDir: dir,
			Cols:    cols,
			Rows:    rows,
		})
		if attachErr != nil {
			output.Error(os.Stderr, fmt.Sprintf("%s: %v", job.Name, attachErr))
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
