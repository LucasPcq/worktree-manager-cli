package runview

import (
	"sync"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// paneSource is where a pane's bytes come from. A pane only ever has one: the
// daemon replays a job's history when a stream is attached, so a pane fed by
// anything else is rebuilt rather than appended to, or the replay would show
// twice.
type paneSource int

const (
	sourceLive paneSource = iota
	sourceHistory
	// sourceSequence is a job whose bytes come from the run starting it, which
	// reports them itself rather than through a subscription.
	sourceSequence
)

type jobPane struct {
	pane   *Pane
	source paneSource
	stream runlogs.Stream
}

// paneStore holds the panes of the jobs on screen. The goroutine reading a
// stream writes into a pane as the bytes arrive while the render loop reads it,
// so the map is guarded here rather than left to Bubbletea's copies of the
// model.
type paneStore struct {
	mu    sync.Mutex
	size  PaneSize
	panes map[string]*jobPane
}

func newPaneStore(size PaneSize) *paneStore {
	return &paneStore{size: size, panes: map[string]*jobPane{}}
}

// open returns the job's pane, building a fresh one when the job has none or
// when its bytes are about to come from somewhere else than they did.
func (s *paneStore) open(job string, source paneSource) *Pane {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openLocked(job, source)
}

func (s *paneStore) openLocked(job string, source paneSource) *Pane {
	if entry, held := s.panes[job]; held && entry.source == source {
		return entry.pane
	}
	// A pane being replaced may still be subscribed to its job; dropping it
	// without closing that would leave the job feeding a pane nobody reads.
	s.closeStreamLocked(job)

	pane := NewPane(PaneParams{Size: s.size})
	s.panes[job] = &jobPane{pane: pane, source: source}
	return pane
}

type writeChunkParams struct {
	Job    string
	Source paneSource
	Chunk  []byte
}

func (s *paneStore) write(params writeChunkParams) {
	s.open(params.Job, params.Source).Write(params.Chunk)
}

func (s *paneStore) writeLines(job string, lines []string) {
	pane := s.open(job, sourceHistory)
	for _, line := range lines {
		pane.Write([]byte(line + "\r\n"))
	}
}

// attach binds a stream to a fresh pane and returns it for the goroutine that
// will feed it. Any stream the job already had is closed first: two
// subscriptions on one job would double its output and fight over its PTY size.
func (s *paneStore) attach(job string, stream runlogs.Stream) *Pane {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeStreamLocked(job)

	pane := NewPane(PaneParams{Size: s.size})
	s.panes[job] = &jobPane{pane: pane, source: sourceLive, stream: stream}
	return pane
}

// release closes a job's stream and drops the pane it fed. The pane is what
// makes it worth doing: a styled cell per column it was ever written, which at
// the scrollback the panes hold runs to megabytes each. Nothing is lost with
// it — the daemon replays a live job's recent output on the next attach, and a
// stopped one's log file is read back in one call. The one pane worth keeping
// is the one a run is writing into right now, which the caller keeps for
// itself: nothing replays that yet.
func (s *paneStore) release(job string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, held := s.panes[job]; !held {
		return
	}
	s.closeStreamLocked(job)
	delete(s.panes, job)
}

// endStream drops a subscription whose job stopped writing, keeping the pane:
// what the job printed before it went is the last thing worth reading, and the
// log file has nothing more to add to it.
func (s *paneStore) endStream(job string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeStreamLocked(job)
}

func (s *paneStore) closeStreamLocked(job string) {
	entry, held := s.panes[job]
	if !held || entry.stream == nil {
		return
	}
	entry.stream.Close()
	entry.stream = nil
}

func (s *paneStore) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for job := range s.panes {
		s.closeStreamLocked(job)
	}
	s.panes = map[string]*jobPane{}
}

func (s *paneStore) entry(job string) (jobPane, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held, ok := s.panes[job]
	if !ok {
		return jobPane{}, false
	}
	return *held, true
}

// stream is the subscription a surface writes to, present only while the job is
// attached.
func (s *paneStore) stream(job string) runlogs.Stream {
	entry, ok := s.entry(job)
	if !ok {
		return nil
	}
	return entry.stream
}

// hasStream reports whether anything is still feeding a pane, which is the
// only reason to keep redrawing on a clock.
func (s *paneStore) hasStream() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.panes {
		if entry.stream != nil {
			return true
		}
	}
	return false
}

// resize sizes every pane the store holds and reports the streams whose job now
// draws at a different size. Resizing a PTY is a round-trip to the daemon, so it
// is left to the caller to make outside this lock.
func (s *paneStore) resize(size PaneSize) []runlogs.Stream {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = size

	var changed []runlogs.Stream
	for _, entry := range s.panes {
		if entry.pane.Resize(size) && entry.stream != nil {
			changed = append(changed, entry.stream)
		}
	}
	return changed
}
