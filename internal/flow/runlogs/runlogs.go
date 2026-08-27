// Package runlogs is the seam between a surface showing a worktree's jobs and
// the daemon holding them: what a job looks like, how its live output is
// subscribed to, and how a profile's start sequence is reported.
package runlogs

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobView struct {
	Name      string
	Kind      domain.JobKind
	Status    domain.JobStatus
	StartedAt time.Time
	ExitCode  *int
	// Attachable is false for a job with no live output to bind to: a detached
	// launcher, whose stream ended with the launcher itself, and any job that is
	// no longer running. History is what is left to show for those. A task is
	// attachable while it runs — the daemon streams it like any other job, and
	// its pane is the only place its colours and redraws survive.
	Attachable bool
}

type Size struct {
	Cols int
	Rows int
}

type Stream interface {
	// Chunks carries the job's bytes exactly as the job wrote them, escape
	// sequences included — an emulator needs them untouched. A chunk is shared
	// with the job's other subscribers: read it, never write into it. The channel
	// closes when the output ends or the stream is closed.
	Chunks() <-chan []byte
	// Write feeds the job's stdin. Nothing arbitrates the PTY between
	// subscribers: two of them writing at once interleave their bytes.
	Write(p []byte) error
	// Resize sizes the job's PTY, the only way its child ever reflows. Every
	// subscriber sizes for itself, so the last one to call wins for all of them.
	Resize(Size) error
	Close() error
}

type AttachParams struct {
	Job string
	// Size sizes the job's PTY as the subscription opens. Zero leaves it as it is.
	Size Size
}

type HistoryParams struct {
	Job   string
	Lines int
}

type Session interface {
	Jobs() []JobView
	// Refresh re-reads the daemon's view of the jobs.
	Refresh() error
	// Attach subscribes to a job's live output, and refuses one that has none.
	Attach(AttachParams) (Stream, error)
	// History reads back the lines a job persisted, whether or not it still runs.
	History(HistoryParams) ([]string, error)
}

type StartRequest struct {
	Job     domain.JobConfig
	WorkDir string
	LogDir  string
	Env     map[string]string
	// OnOutput receives what the job writes while it starts — everything for a
	// task or a detached launcher, nothing for a job the daemon backgrounds.
	OnOutput func([]byte)
}

// StartResult is the daemon's answer. Refused is the daemon declining to start
// the job, as opposed to the error Start returns when it could not be reached.
type StartResult struct {
	Refused bool
	Message string
	// ExitCode is what a task exited with, nil for a job whose lifetime does not
	// end with this answer.
	ExitCode *int
	// Ports are the ports the job bound, base plus this worktree's offset. Empty
	// for a job that declares none.
	Ports map[string]int
}

type AttachRequest struct {
	Name    string
	WorkDir string
	Size    Size
}

type TailRequest struct {
	LogDir string
	Job    string
	Lines  int
}

// Service is the daemon as this package uses it: the seam NewService implements
// over internal/service/process, and a test replaces.
type Service interface {
	// Start returns as soon as the run stops watching: cancelling ends the
	// conversation about the job, never the job.
	Start(context.Context, StartRequest) (StartResult, error)
	List(workDir string) ([]domain.JobInfo, error)
	Attach(AttachRequest) (Stream, error)
	Tail(TailRequest) ([]string, error)
}

type Phase int

const (
	// PhaseStarting through PhaseDone are one job's own steps; PhaseAborted and
	// PhaseReady conclude the sequence and carry its Outcome.
	PhaseStarting Phase = iota
	PhaseOutput
	PhaseStarted
	PhaseDone
	PhaseFailed
	PhaseAborted
	PhaseProbed
	PhaseReady
)

// Prober answers which of the given ports are listening. It is the seam over
// service/portprobe: the flow states what it wants checked, the surface owns
// the budget and the dialing.
type Prober interface {
	// Listening dials until settled says the answer is complete or the surface's
	// own budget runs out.
	Listening(ctx context.Context, ports []int, settled func(map[int]bool) bool) map[int]bool
}

// Event is one step of a profile's start sequence. Rendering it is the surface's
// business: nothing here formats, colours or frames anything.
type Event struct {
	Phase Phase
	Job   string
	// Kind is the job's, carried on PhaseOutput so a surface knows what it is
	// reading: a task's bytes come off a pipe, with none of the line discipline
	// a PTY applies on the way out.
	Kind domain.JobKind
	// Step places the job in the sequence, from 1 to Steps.
	Step  int
	Steps int
	// Chunk is a PhaseOutput's raw bytes, under the read-only contract Stream
	// states.
	Chunk []byte
	// Reason is what the daemon answered when a job could not be started.
	Reason string
	// AlreadyRunning marks a start refused because the job was already up, which
	// is benign: it counts as running and the sequence carries on.
	AlreadyRunning bool
	// Outcome is the partial state PhaseAborted reports and the final state
	// PhaseReady reports; zero on every other phase.
	Outcome Outcome
	// Ports are what a PhaseStarted or PhaseDone job bound, for a surface that
	// tells the user where to reach it.
	Ports map[string]int
	// URL is where a PhaseStarted or PhaseDone job is reachable, empty for one
	// that publishes no name.
	URL string
	// Probes is what PhaseProbed observed on one job's declared ports.
	Probes []domain.PortProbe
}

// Sink is emitted to on the goroutine that called Run.
type Sink interface {
	Emit(Event)
}

type noSink struct{}

func (noSink) Emit(Event) {}
