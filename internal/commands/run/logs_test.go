package run

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func linesCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	return cmd, &out, &errOut
}

func fedStream(t *testing.T, chunks ...string) *runlogstest.Stream {
	t.Helper()

	stream := runlogstest.NewStream()
	for _, chunk := range chunks {
		stream.Feed([]byte(chunk))
	}
	// The reader stops at the end of the output, which is what lets the command
	// return instead of waiting on a job that never ends.
	stream.Close()
	return stream
}

func TestWriteJobLinesPrefixesEveryJob(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true},
			{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true},
		},
		Streams: map[string]runlogs.Stream{
			"api": fedStream(t, "listening on 3000\n"),
			"web": fedStream(t, "ready in 412ms\n"),
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	for _, want := range []string{"[api] listening on 3000", "[web] ready in 412ms"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, out.String())
		}
	}
}

// Every line carries its prefix, not just the first one of a chunk: without it
// a job printing a page is indistinguishable from the job beside it.
func TestWriteJobLinesPrefixesEveryLineOfAChunk(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views:   []runlogs.JobView{{Name: "api", Status: domain.JobStatusRunning, Attachable: true}},
		Streams: map[string]runlogs.Stream{"api": fedStream(t, "first\nsecond\n", "third\n")},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if got := strings.Count(out.String(), "[api]"); got != 3 {
		t.Errorf("counted %d prefixes for 3 lines\n--- stdout ---\n%s", got, out.String())
	}
}

// A job that is not running has no stream to attach to, and its log file is the
// only place its output is left.
func TestWriteJobLinesReadsAStoppedJobBackFromItsLogFile(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped}},
		Lines: map[string][]string{"migrate": {"applying 001", "done"}},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session, Job: "migrate"}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	for _, want := range []string{"[migrate] applying 001", "[migrate] done"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, out.String())
		}
	}
}

func TestWriteJobLinesNarrowsToTheNamedJob(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{
			{Name: "api", Status: domain.JobStatusRunning, Attachable: true},
			{Name: "web", Status: domain.JobStatusRunning, Attachable: true},
		},
		Streams: map[string]runlogs.Stream{
			"api": fedStream(t, "listening on 3000\n"),
			"web": fedStream(t, "ready in 412ms\n"),
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session, Job: "api"}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if strings.Contains(out.String(), "web") {
		t.Errorf("naming a job did not narrow the output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "[api] listening on 3000") {
		t.Errorf("stdout is missing the named job's line:\n%s", out.String())
	}
}

func TestWriteJobLinesRefusesAJobTheWorktreeDoesNotHave(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{{Name: "api", Status: domain.JobStatusRunning, Attachable: true}},
	})

	cmd, _, _ := linesCmd()
	err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session, Job: "ghost"})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want one naming the unknown job", err)
	}
}

func TestWriteJobLinesSaysSoWhenNothingIsRunning(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{{Name: "api", Status: domain.JobStatusStopped}},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Session: session}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if !strings.Contains(out.String(), domain.RunLogsNoJobs) {
		t.Errorf("stdout does not say the worktree has nothing running:\n%s", out.String())
	}
}

func TestRunLogsOpensTheViewOnATerminal(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdLogs, "--"+domain.FlagJob, "api"); err != nil {
		t.Fatalf("run logs api: %v", err)
	}

	call := view.only(t)
	if call.Job != "api" {
		t.Errorf("the view opened on %q, want api", call.Job)
	}
	// `run logs` reports on what is already running: it starts nothing, so it
	// has no recap to print on the way out.
	if call.Attached {
		t.Error("run logs handed the view a start sequence")
	}
}

func TestRunLogsWithoutATerminalWritesPrefixedLines(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	setupStartProject(t, &fakeDaemon{
		Jobs: []domain.JobInfo{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: dir},
		},
		Streams: map[string][]byte{"api": []byte("listening on 3000\nrequest handled\n")},
	})
	view := captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdLogs)
	if err != nil {
		t.Fatalf("run logs: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a pipe opened the view: %+v", view.calls)
	}
	for _, want := range []string{"[api] listening on 3000", "[api] request handled"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
}
