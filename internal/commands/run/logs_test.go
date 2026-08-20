package run

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func runningView(name string) runlogs.JobView {
	return runlogs.JobView{Name: name, Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true}
}

func lines(out string) []string {
	return strings.Split(strings.Trim(out, "\n"), "\n")
}

// TestStreamJobLinesPrintsWholeSanitizedLines is what a pipe gets instead of a
// pane: no escape sequence survives, a line split across two reads is printed
// once, and a redrawn progress bar leaves only what it settled on.
func TestStreamJobLinesPrintsWholeSanitizedLines(t *testing.T) {
	stream := runlogstest.NewStream()
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views:   []runlogs.JobView{runningView("api")},
		Streams: map[string]runlogs.Stream{"api": stream},
	})

	stream.Feed([]byte("\x1b[32mready in 312ms\x1b[0m\r\nbuil"))
	stream.Feed([]byte("ding…\nbuild 1/3\rbuild 2/3\rbuild 3/3\n"))
	stream.Close()

	var out bytes.Buffer
	if err := streamJobLines(logLinesParams{Out: &out, Err: &out, Session: session}); err != nil {
		t.Fatalf("streamJobLines: %v", err)
	}

	want := []string{"[api] ready in 312ms", "[api] building…", "[api] build 3/3"}
	if got := lines(out.String()); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
}

// TestStreamJobLinesPrintsTheTailWithoutItsNewline keeps a prompt or a last
// line a job never terminated from being swallowed when its stream ends.
func TestStreamJobLinesPrintsTheTailWithoutItsNewline(t *testing.T) {
	stream := runlogstest.NewStream()
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views:   []runlogs.JobView{runningView("api")},
		Streams: map[string]runlogs.Stream{"api": stream},
	})

	stream.Feed([]byte("listening on :3000"))
	stream.Close()

	var out bytes.Buffer
	if err := streamJobLines(logLinesParams{Out: &out, Err: &out, Session: session}); err != nil {
		t.Fatalf("streamJobLines: %v", err)
	}

	if got := lines(out.String()); !reflect.DeepEqual(got, []string{"[api] listening on :3000"}) {
		t.Fatalf("lines = %q, want the unterminated tail", got)
	}
}

func TestStreamJobLinesNamesEveryRunningJob(t *testing.T) {
	api, web := runlogstest.NewStream(), runlogstest.NewStream()
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{
			runningView("api"),
			{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped},
			runningView("web"),
		},
		Streams: map[string]runlogs.Stream{"api": api, "web": web},
	})

	api.Feed([]byte("api up\n"))
	api.Close()
	web.Feed([]byte("web up\n"))
	web.Close()

	var out bytes.Buffer
	if err := streamJobLines(logLinesParams{Out: &out, Err: &out, Session: session}); err != nil {
		t.Fatalf("streamJobLines: %v", err)
	}

	got := out.String()
	for _, want := range []string{"[api] api up", "[web] web up"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "migrate") {
		t.Errorf("a stopped job was attached to\n--- output ---\n%s", got)
	}
	if session.Refreshes() != 1 {
		t.Errorf("refreshes = %d, want the daemon read once", session.Refreshes())
	}
}

func TestStreamJobLinesReadsANamedStoppedJobBack(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{
			runningView("api"),
			{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped},
		},
		Lines: map[string][]string{"migrate": {"2026-08-20T12:04:11Z  applied 3 migrations"}},
	})

	var out bytes.Buffer
	if err := streamJobLines(logLinesParams{Out: &out, Err: &out, Session: session, Job: "migrate"}); err != nil {
		t.Fatalf("streamJobLines: %v", err)
	}

	if got := lines(out.String()); !reflect.DeepEqual(got, []string{"[migrate] 2026-08-20T12:04:11Z  applied 3 migrations"}) {
		t.Fatalf("lines = %q, want the persisted history", got)
	}
	if got := session.AttachedJobs(); len(got) != 0 {
		t.Errorf("attached %v, want a job with no live output read from its file", got)
	}
}

func TestStreamJobLinesSaysWhenNothingRuns(t *testing.T) {
	session := runlogstest.NewSession(runlogstest.SessionParams{
		Views: []runlogs.JobView{{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
	})

	var out bytes.Buffer
	if err := streamJobLines(logLinesParams{Out: &out, Err: &out, Session: session}); err != nil {
		t.Fatalf("streamJobLines: %v", err)
	}

	if !strings.Contains(out.String(), domain.RunNoRunningJobs) {
		t.Errorf("output = %q, want %q", out.String(), domain.RunNoRunningJobs)
	}
}

func TestSelectLogJobs(t *testing.T) {
	views := []runlogs.JobView{
		runningView("api"),
		{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped},
		runningView("web"),
	}

	tests := []struct {
		name string
		job  string
		want []string
	}{
		{"every running job", "", []string{"api", "web"}},
		{"the one named", "web", []string{"web"}},
		{"a named job that stopped", "migrate", []string{"migrate"}},
		{"a job nobody declared", "ghost", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, view := range selectLogJobs(views, tt.job) {
				got = append(got, view.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectLogJobs(%q) = %v, want %v", tt.job, got, tt.want)
			}
		})
	}
}
