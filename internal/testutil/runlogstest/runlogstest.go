// Package runlogstest provides doubles for the run log seam: a Service that
// answers from a script instead of a daemon, a Stream fed by hand, and a Sink
// that records what a run emitted.
package runlogstest

import (
	"fmt"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

type StartCall struct {
	Job     domain.JobConfig
	WorkDir string
	LogDir  string
}

type Service struct {
	// Refusals maps a job name to the message the daemon answers instead of
	// starting it, Errors to a daemon it could not reach at all, and ExitCodes
	// to the code the daemon reports alongside a refusal.
	Refusals  map[string]string
	Errors    map[string]error
	ExitCodes map[string]int
	// Output maps a job name to what it writes while it starts.
	Output map[string][]string

	Infos   []domain.JobInfo
	ListErr error

	Streams   map[string]runlogs.Stream
	AttachErr error

	// Lines maps a job name to its persisted history.
	Lines map[string][]string

	Started  []StartCall
	Attached []runlogs.AttachRequest
	Tailed   []runlogs.TailRequest
}

func (s *Service) Start(req runlogs.StartRequest) (runlogs.StartResult, error) {
	s.Started = append(s.Started, StartCall{Job: req.Job, WorkDir: req.WorkDir, LogDir: req.LogDir})

	if req.OnOutput != nil {
		for _, chunk := range s.Output[req.Job.Name] {
			req.OnOutput([]byte(chunk))
		}
	}
	if err := s.Errors[req.Job.Name]; err != nil {
		return runlogs.StartResult{}, err
	}
	if message, refused := s.Refusals[req.Job.Name]; refused {
		result := runlogs.StartResult{Refused: true, Message: message}
		if code, scripted := s.ExitCodes[req.Job.Name]; scripted {
			result.ExitCode = &code
		}
		return result, nil
	}
	return runlogs.StartResult{}, nil
}

func (s *Service) List(string) ([]domain.JobInfo, error) {
	if s.ListErr != nil {
		return nil, s.ListErr
	}
	return s.Infos, nil
}

func (s *Service) Attach(req runlogs.AttachRequest) (runlogs.Stream, error) {
	s.Attached = append(s.Attached, req)
	if s.AttachErr != nil {
		return nil, s.AttachErr
	}
	stream, ok := s.Streams[req.Name]
	if !ok {
		return nil, fmt.Errorf("no stream scripted for job %q", req.Name)
	}
	return stream, nil
}

func (s *Service) Tail(req runlogs.TailRequest) ([]string, error) {
	s.Tailed = append(s.Tailed, req)
	return s.Lines[req.Job], nil
}

// StartedNames joins the jobs the run asked to start, for a one-line assertion.
func (s *Service) StartedNames() []string {
	names := make([]string, 0, len(s.Started))
	for _, call := range s.Started {
		names = append(names, call.Job.Name)
	}
	return names
}

type Sink struct {
	Events []runlogs.Event
}

func (s *Sink) Emit(event runlogs.Event) { s.Events = append(s.Events, event) }

func (s *Sink) Phases() []runlogs.Phase {
	phases := make([]runlogs.Phase, 0, len(s.Events))
	for _, event := range s.Events {
		phases = append(phases, event.Phase)
	}
	return phases
}

// Last returns the last event of a phase, which is where a run's conclusion is.
func (s *Sink) Last(phase runlogs.Phase) (runlogs.Event, bool) {
	for i := len(s.Events) - 1; i >= 0; i-- {
		if s.Events[i].Phase == phase {
			return s.Events[i], true
		}
	}
	return runlogs.Event{}, false
}

// Stream is a job's output under the test's control: Feed delivers a chunk, and
// what a surface writes back is recorded rather than sent anywhere.
type Stream struct {
	chunks chan []byte

	mu      sync.Mutex
	written [][]byte
	sizes   []runlogs.Size
	closed  bool
}

func NewStream() *Stream {
	return &Stream{chunks: make(chan []byte, domain.JobStreamQueueChunks)}
}

func (s *Stream) Chunks() <-chan []byte { return s.chunks }

func (s *Stream) Feed(chunk []byte) { s.chunks <- chunk }

func (s *Stream) Write(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written = append(s.written, append([]byte(nil), p...))
	return nil
}

func (s *Stream) Resize(size runlogs.Size) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sizes = append(s.sizes, size)
	return nil
}

func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.chunks)
	return nil
}

func (s *Stream) Written() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	written := make([]string, 0, len(s.written))
	for _, chunk := range s.written {
		written = append(written, string(chunk))
	}
	return written
}

func (s *Stream) Sizes() []runlogs.Size {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]runlogs.Size(nil), s.sizes...)
}

func (s *Stream) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
