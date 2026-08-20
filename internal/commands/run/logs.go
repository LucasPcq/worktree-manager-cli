package run

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

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

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdLogs + " [job]",
		Short: "Open the run view on this worktree's jobs",
		Long:  "Open the run view: one pane per job, its output live, Ctrl+C detaches without stopping anything.\nWith a job name, the view opens on that job.\nWithout a terminal, streams the running jobs as prefixed lines instead.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runLogs,
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
		Message: "Connecting to daemon…",
		Animate: true,
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	job := ""
	if len(args) > 0 {
		job = args[0]
	}

	surface := surfaceParams{
		Out:      cmd.OutOrStdout(),
		Err:      cmd.ErrOrStderr(),
		Format:   domain.OutputText,
		Service:  runlogs.NewService(runlogs.ServiceParams{SocketPath: socketPath}),
		Declared: runCfg.Jobs,
		Focus:    job,
		WorkDir:  dir,
		LogDir:   jobLogDir(jobLogDirParams{StateDir: result.StateDir, Dir: dir}),
	}

	if wantsRunView(wantsRunViewParams{Format: domain.OutputText}) {
		_, err := startInView(surface)
		return err
	}

	return streamJobLines(logLinesParams{
		Out:     surface.Out,
		Err:     surface.Err,
		Session: newSession(surface),
		Job:     job,
	})
}

// jobColors cycles through distinct colors for each job's log prefix.
var jobColors = []func(string) string{
	func(s string) string { return styles.Primary.Render(s) },
	func(s string) string { return styles.Success.Render(s) },
	func(s string) string { return styles.Warning.Render(s) },
	func(s string) string { return styles.Muted.Render(s) },
}

type logLinesParams struct {
	Out io.Writer
	Err io.Writer
	// Session is the worktree's jobs; Job narrows it to one, empty takes every
	// job that is running.
	Session runlogs.Session
	Job     string
}

// streamJobLines is what a reader that is not a terminal gets instead of the
// run view: every job on one stream, sanitized of the escape sequences a pane
// would have drawn, one line at a time behind the name of the job that printed
// it. A job with no live output is read back from its log file.
func streamJobLines(params logLinesParams) error {
	if err := params.Session.Refresh(); err != nil {
		return err
	}

	views := selectLogJobs(params.Session.Jobs(), params.Job)
	if len(views) == 0 {
		output.Frame(params.Out, func() { output.Message(params.Out, domain.RunNoRunningJobs) })
		return nil
	}

	output.FrameStart(params.Out)
	defer output.FrameEnd(params.Out)

	writer := &lineWriter{out: params.Out}
	var wg sync.WaitGroup
	for index, view := range views {
		prefix := jobColors[index%len(jobColors)](fmt.Sprintf(domain.RunLogPrefixFmt, view.Name))

		if !view.Attachable {
			lines, err := params.Session.History(runlogs.HistoryParams{Job: view.Name})
			if err != nil {
				output.Error(params.Err, fmt.Sprintf("%s: %v", view.Name, err))
				continue
			}
			writer.lines(prefix, lines)
			continue
		}

		stream, err := params.Session.Attach(runlogs.AttachParams{Job: view.Name})
		if err != nil {
			output.Error(params.Err, fmt.Sprintf("%s: %v", view.Name, err))
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stream.Close()
			tailStream(tailStreamParams{Stream: stream, Prefix: prefix, Writer: writer})
		}()
	}

	wg.Wait()
	return nil
}

// selectLogJobs is what a named job answers for itself — whether or not it is
// still running, its log file is there to read — against the whole worktree,
// where only a running job has anything left to say.
func selectLogJobs(views []runlogs.JobView, job string) []runlogs.JobView {
	if job != "" {
		for _, view := range views {
			if view.Name == job {
				return []runlogs.JobView{view}
			}
		}
		return nil
	}

	running := make([]runlogs.JobView, 0, len(views))
	for _, view := range views {
		if view.Status == domain.JobStatusRunning {
			running = append(running, view)
		}
	}
	return running
}

type tailStreamParams struct {
	Stream runlogs.Stream
	Prefix string
	Writer *lineWriter
}

// tailStream prints a job's output as whole lines. The tail a chunk ends on is
// carried to the next one: a line split across two reads is one line here, and
// a progress bar redrawn over itself is the value it settled on.
func tailStream(params tailStreamParams) {
	pending := ""
	for chunk := range params.Stream.Chunks() {
		result := rules.SanitizeLogChunk(rules.SanitizeChunkParams{
			Chunk:   string(chunk),
			Pending: pending,
			At:      time.Now(),
		})
		pending = result.Pending
		for _, record := range result.Records {
			params.Writer.line(params.Prefix, record.Text)
		}
	}

	if text := rules.SanitizeLogLine(pending); text != "" {
		params.Writer.line(params.Prefix, text)
	}
}

// lineWriter is the one stream every job writes to: a line lands whole or not
// at all, however many jobs are being read at once.
type lineWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *lineWriter) line(prefix string, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.out, "%s %s\n", prefix, text)
}

func (w *lineWriter) lines(prefix string, texts []string) {
	for _, text := range texts {
		w.line(prefix, text)
	}
}
