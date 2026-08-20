package run

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newLogsCmd creates the wtm run logs subcommand.
func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdLogs + " [job]",
		Short: "Attach to a job's output",
		Long: "Open the run view on this worktree's jobs, focused on [job] when one is named.\n" +
			"Leaving the view detaches; the jobs keep running.\n" +
			"Without a terminal, every job's output is written as prefixed lines instead.",
		Args: cobra.MaximumNArgs(1),
		RunE: runLogs,
	}
}

func runLogs(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return err
	}

	socketPath := process.SocketPath()
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.RunDaemonConnecting,
		Animate: true,
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	job := ""
	if len(args) > 0 {
		job = args[0]
	}

	seam := openRunSeam(runSeamParams{StateDir: result.StateDir, Dir: dir, Jobs: runCfg.Jobs})
	surface := rules.DecideRunSurface(rules.RunSurfaceParams{TTY: isTTY(), Format: domain.OutputText})
	if surface == domain.RunSurfaceView {
		return showRunView(viewParams{Cmd: cmd, Session: seam.session, Job: job})
	}

	return writeJobLines(jobLinesParams{Cmd: cmd, Session: seam.session, Job: job})
}

// jobColors cycles through distinct colors for each job's log prefix.
var jobColors = []func(string) string{
	func(s string) string { return styles.Primary.Render(s) },
	func(s string) string { return styles.Success.Render(s) },
	func(s string) string { return styles.Warning.Render(s) },
	func(s string) string { return styles.Muted.Render(s) },
}

type jobLinesParams struct {
	Cmd     *cobra.Command
	Session runlogs.Session
	// Job narrows the output to one job; empty takes every job the worktree has.
	Job string
}

// writeJobLines is `run logs` with no terminal to draw on: one prefixed line per
// job on a single stream, live while the job runs and read back from its log
// file when it does not. Nothing here touches the terminal's mode — the process
// is left to die on SIGINT like any other pipe.
func writeJobLines(params jobLinesParams) error {
	if err := params.Session.Refresh(); err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	views, err := logJobViews(params)
	if err != nil {
		return err
	}
	out := params.Cmd.OutOrStdout()
	if len(views) == 0 {
		output.Frame(out, func() { output.Message(out, domain.RunLogsNoJobs) })
		return nil
	}

	output.FrameStart(out)
	writer := &lineWriter{out: out}
	var wg sync.WaitGroup

	for i, view := range views {
		prefix := jobColors[i%len(jobColors)](fmt.Sprintf(domain.RunLogsPrefixFmt, view.Name))

		if !view.Attachable {
			lines, historyErr := params.Session.History(runlogs.HistoryParams{Job: view.Name})
			if historyErr != nil {
				output.Error(params.Cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", view.Name, historyErr))
				continue
			}
			for _, line := range lines {
				writer.write(prefix, line)
			}
			continue
		}

		stream, attachErr := params.Session.Attach(runlogs.AttachParams{Job: view.Name})
		if attachErr != nil {
			output.Error(params.Cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", view.Name, attachErr))
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stream.Close()
			scanner := bufio.NewScanner(&streamReader{chunks: stream.Chunks()})
			for scanner.Scan() {
				writer.write(prefix, scanner.Text())
			}
		}()
	}

	wg.Wait()
	output.FrameEnd(out)
	return nil
}

// logJobViews is what this run of `run logs` reports on: the named job, or every
// job the worktree has anything to show for.
func logJobViews(params jobLinesParams) ([]runlogs.JobView, error) {
	jobs := params.Session.Jobs()
	if params.Job != "" {
		for _, view := range jobs {
			if view.Name == params.Job {
				return []runlogs.JobView{view}, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
	}

	views := make([]runlogs.JobView, 0, len(jobs))
	for _, view := range jobs {
		if view.Attachable {
			views = append(views, view)
		}
	}
	return views, nil
}

// lineWriter serializes the lines several jobs write at once: without it two
// prefixes and two lines interleave inside one row.
type lineWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *lineWriter) write(prefix string, line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.out, "%s%s %s\n", output.Indent, prefix, line)
}

// streamReader reads a job's chunks as the io.Reader a line scanner needs.
type streamReader struct {
	chunks  <-chan []byte
	pending []byte
}

// Read never writes into a chunk: it is shared with the job's other
// subscribers, so what does not fit in p is kept as a slice of it until the
// next call.
func (r *streamReader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		chunk, open := <-r.chunks
		if !open {
			return 0, io.EOF
		}
		r.pending = chunk
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
