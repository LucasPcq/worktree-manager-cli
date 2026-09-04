package runview

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// program is Bubbletea's loop, reduced to what this model needs from it: every
// command runs on its own goroutine, batches are unrolled, and what a command
// returns comes back as the next message. Driving Update by hand hides the one
// thing that matters here — a model that stops re-arming its listen goes deaf,
// and the run behind it stalls on a full mailbox rather than failing.
type program struct {
	t *testing.T

	mu    sync.Mutex
	model Model

	queue chan tea.Msg
	done  chan struct{}
	once  sync.Once
}

func newProgram(t *testing.T, model Model) *program {
	t.Helper()
	p := &program{
		t:     t,
		model: model,
		queue: make(chan tea.Msg, domain.RunViewMsgBuffer*4),
		done:  make(chan struct{}),
	}
	t.Cleanup(p.stop)
	go p.loop()
	p.exec(model.Init())
	return p
}

func (p *program) loop() {
	for {
		select {
		case <-p.done:
			return
		case msg := <-p.queue:
			p.mu.Lock()
			next, cmd := p.model.Update(msg)
			p.model = next.(Model)
			p.mu.Unlock()
			p.exec(cmd)
		}
	}
}

func (p *program) exec(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		batch, isBatch := msg.(tea.BatchMsg)
		if !isBatch {
			p.send(msg)
			return
		}
		for _, sub := range batch {
			p.exec(sub)
		}
	}()
}

func (p *program) send(msg tea.Msg) {
	if msg == nil {
		return
	}
	if _, quit := msg.(tea.QuitMsg); quit {
		p.stop()
		return
	}
	select {
	case p.queue <- msg:
	case <-p.done:
	}
}

func (p *program) stop() {
	p.once.Do(func() {
		close(p.done)
		p.mu.Lock()
		defer p.mu.Unlock()
		p.model.cancel()
		p.model.panes.closeAll()
	})
}

// waitFor is how a test observes a loop it does not step through.
func (p *program) waitFor(what string, reached func(Model) bool) {
	p.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		done := reached(p.model)
		p.mu.Unlock()
		if done {
			return
		}
		time.Sleep(time.Millisecond)
	}
	p.t.Fatalf("timed out waiting for %s", what)
}

// The run and the stream readers share one channel, and the model holds the
// only read on it. Taking a message without asking for the next leaves it deaf
// after a single event: the sink fills the mailbox and blocks, and the profile
// stops mid-sequence with nothing said.
func TestTheModelKeepsHearingFromARunPastTheMailbox(t *testing.T) {
	const steps = domain.RunViewMsgBuffer * 3

	finished := make(chan struct{})
	model := New(Params{
		Board: runlogstest.NewBoard(runlogstest.BoardParams{
			Views: []runlogs.JobView{stopped("migrate")},
		}),
		Start: func(_ context.Context, emitter runlogs.Sink) (runlogs.Outcomes, error) {
			defer close(finished)
			for step := 1; step <= steps; step++ {
				emitter.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "migrate", Step: step, Steps: steps})
				emitter.Emit(runlogs.Event{Phase: runlogs.PhaseDone, Job: "migrate", Step: step, Steps: steps})
			}
			return runlogs.Outcomes{runlogs.Outcome{Steps: steps}}, nil
		},
	})

	p := newProgram(t, model)
	p.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the run stalled: the model stopped taking what the sink posts")
	}
	p.waitFor("the last step of the run to reach the model", func(m Model) bool {
		return m.sequence.step == steps
	})
}

// The end of a stream travels on that same channel. A model that takes one
// without asking for the next never hears from any reader behind it: their
// panes keep a subscription nothing feeds, and the redraw clock keeps running.
func TestTheModelKeepsHearingFromTheReadersPastAStreamEnding(t *testing.T) {
	first, second := runlogstest.NewStream(), runlogstest.NewStream()
	views := []runlogs.JobView{running("api"), running("web")}
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views:   views,
		Streams: map[string]runlogs.Stream{"api": first, "web": second},
	})

	p := newProgram(t, New(Params{Board: board}))
	p.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	p.waitFor("api to be attached", func(m Model) bool { return m.panes.stream("api") != nil })

	first.Close()
	p.waitFor("the end of api's stream", func(m Model) bool { return m.panes.stream("api") == nil })

	p.send(namedKey(tea.KeyDown))
	p.waitFor("web to be attached", func(m Model) bool { return m.panes.stream("web") != nil })

	second.Close()
	p.waitFor("the end of web's stream", func(m Model) bool { return m.panes.stream("web") == nil })
}
