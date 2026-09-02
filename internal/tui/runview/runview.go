package runview

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Params struct {
	Board runlogs.Board
	// Job is selected when the view opens; empty takes the first one.
	Job string
	// Profile names what Start brings up, shown in the header and the recap.
	// Empty for a view that starts nothing, or a run.toml declaring no profile.
	Profile string
	// Start runs a profile's start sequence while the view is open, reporting to
	// the Sink it is given. Nil for a view that only reads what is already
	// running. Cancelling the context ends the reporting, never the jobs.
	Start StartFunc
	// Open hands a job's URL to the desktop. Nil leaves the open key without an
	// object, which is what a surface that cannot open a browser installs.
	Open OpenFunc
}

// OpenFunc opens a URL outside the terminal. The view never dials anything
// itself: it names what to open and lets the seam do it.
type OpenFunc func(url string) error

// StartFunc is a start sequence the view drives — runlogs.Run, wired by the
// command that opened the view.
type StartFunc func(context.Context, runlogs.Sink) (runlogs.Outcome, error)

// Model is the run view's root Bubbletea model: a job list, the selected job's
// terminal emulator, and nothing else. What a job is, whether it can be
// attached to and what it has printed are runlogs' answers, never its own.
type Model struct {
	board runlogs.Board
	panes *paneStore
	// msgs carries what the stream readers post; listenCmd is its only reader.
	msgs chan tea.Msg

	width  int
	height int

	jobs     []runlogs.JobView
	selected string
	offset   int
	err      error

	filtering bool
	filter    string
	// focused reports that the keyboard belongs to the selected job rather than
	// to this view.
	focused bool

	// lastExitKey dates the last exit key the focused job received, which is
	// what turns a second one into a way out.
	lastExitKey time.Time
	// notice is a refusal the view has to answer with rather than act on.
	notice string

	// pending is the job whose pane is being filled — an attach or a history
	// read in flight — so a second one is not started behind it.
	pending string
	// ticking reports whether a redraw tick is already scheduled: a pane being
	// written to is redrawn on a clock, never once per chunk.
	ticking bool

	start StartFunc
	// profile names the run the view is reporting on, for the header and the recap.
	profile string
	open    OpenFunc
	// started reports that a run was asked for, which is what makes a recap
	// worth printing on the way out.
	started  bool
	sequence sequence
	// following keeps the cursor on the job the sequence is acting on, until the
	// reader takes it themselves.
	following bool
	// dismissed hides the abort report; the outcome it was built from stays.
	dismissed bool

	runCtx context.Context
	cancel context.CancelFunc
}

func New(params Params) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		board:    params.Board,
		selected: params.Job,
		profile:  params.Profile,
		panes:    newPaneStore(PaneSize{}),
		msgs:     make(chan tea.Msg, domain.RunViewMsgBuffer),
		start:    params.Start,
		open:     params.Open,
		started:  params.Start != nil,
		// A run feeds panes from its own goroutine, so the clock has to be
		// running before the first chunk lands.
		ticking:   params.Start != nil,
		following: params.Start != nil,
		sequence:  sequence{states: map[string]domain.JobStep{}},
		runCtx:    ctx,
		cancel:    cancel,
	}
}

// Run opens the view on the alternate screen and returns what to say once it is
// given back. Leaving it detaches: the jobs it was showing keep running, and a
// start sequence it was reporting on carries on without a reader.
func Run(params Params) (Result, error) {
	model := New(params)
	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		model.cancel()
		model.panes.closeAll()
		return Result{}, fmt.Errorf("run view: %w", err)
	}
	last, ok := final.(Model)
	if !ok {
		return Result{}, nil
	}
	return last.result(), nil
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.refreshCmd(), pollCmd(), m.listenCmd()}
	if m.start != nil {
		cmds = append(cmds, m.startCmd(), frameCmd())
	}
	return tea.Batch(cmds...)
}

// startCmd runs the start sequence off the render loop. Its events come back
// through the sink, which writes a job's output straight into that job's pane
// and posts only the phases the view has to draw.
func (m Model) startCmd() tea.Cmd {
	start, ctx := m.start, m.runCtx
	emitter := sink{panes: m.panes, msgs: m.msgs, done: ctx.Done()}
	return func() tea.Msg {
		outcome, err := start(ctx, emitter)
		return runFinishedMsg{outcome: outcome, err: err}
	}
}

type jobsMsg struct {
	jobs []runlogs.JobView
	err  error
}

type attachedMsg struct {
	job    string
	stream runlogs.Stream
	// size is what the job's PTY was sized to as the subscription opened, which
	// the view may have outgrown while the daemon was answering.
	size PaneSize
	err  error
}

type historyMsg struct {
	job   string
	lines []string
	err   error
}

type streamEndedMsg struct{ job string }

type streamErrMsg struct {
	job string
	err error
}

type frameMsg struct{}

type pollMsg struct{}

func (m Model) refreshCmd() tea.Cmd {
	board := m.board
	return func() tea.Msg {
		if err := board.Refresh(); err != nil {
			return jobsMsg{jobs: board.Jobs(), err: err}
		}
		return jobsMsg{jobs: board.Jobs()}
	}
}

type attachParams struct {
	Job  string
	Size PaneSize
}

func (m Model) attachCmd(params attachParams) tea.Cmd {
	board := m.board
	return func() tea.Msg {
		stream, err := board.Attach(runlogs.AttachParams{
			Job:  params.Job,
			Size: runlogs.Size{Cols: params.Size.Cols, Rows: params.Size.Rows},
		})
		return attachedMsg{job: params.Job, stream: stream, size: params.Size, err: err}
	}
}

func (m Model) historyCmd(job string) tea.Cmd {
	board := m.board
	return func() tea.Msg {
		lines, err := board.History(runlogs.HistoryParams{Job: job})
		return historyMsg{job: job, lines: lines, err: err}
	}
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.RunViewPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

func frameCmd() tea.Cmd {
	return tea.Tick(time.Second/domain.RunViewRenderFPS, func(time.Time) tea.Msg { return frameMsg{} })
}

// listenCmd takes the next message the stream readers and the run posted, and
// gives up when the view does: the channel is never closed, so a reader parked
// on it would outlive the program.
func (m Model) listenCmd() tea.Cmd {
	msgs, done := m.msgs, m.runCtx.Done()
	return func() tea.Msg {
		select {
		case msg := <-msgs:
			return msg
		case <-done:
			return nil
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.applySize()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case jobsMsg:
		return m.applyJobs(msg)

	case attachedMsg:
		return m.applyAttached(msg)

	case historyMsg:
		return m.applyHistory(msg)

	case eventMsg:
		return m.applyEvent(msg)

	case runFinishedMsg:
		return m.applyRunFinished(msg)

	case openFailedMsg:
		m.notice = fmt.Sprintf(domain.RunViewOpenFailedFmt, msg.err)
		return m, nil

	case streamErrMsg:
		m.err = msg.err
		return m, nil

	case streamEndedMsg:
		return m.applyStreamEnded(msg)

	case pollMsg:
		return m, tea.Batch(m.refreshCmd(), pollCmd())

	case frameMsg:
		if !m.panes.hasStream() && !m.sequence.active {
			m.ticking = false
			return m, nil
		}
		return m, frameCmd()
	}

	return m, nil
}

// applySize gives the emulators the room the layout just measured, and the
// jobs behind them the same size. x/vt never reflows: only the job's own
// process can redraw at the new width, and it does not know about it until its
// PTY does.
func (m Model) applySize() (Model, tea.Cmd) {
	size := m.paneSize()
	return m, resizeCmd(resizeParams{Streams: m.panes.resize(size), Size: size})
}

// resyncSize re-sizes the emulators when the abort report's occupation of the
// frame changed. The band takes its rows from the body, and a pane fed at one
// size while drawn shorter shows its oldest rows: the output of the job that
// just failed is exactly what would fall off the bottom.
func (m Model) resyncSize(noticeLines int) (Model, tea.Cmd) {
	if len(m.report()) == noticeLines {
		return m, nil
	}
	return m.applySize()
}

func (m Model) paneSize() PaneSize {
	layout := m.layout()
	return PaneSize{Cols: layout.PaneCols, Rows: layout.PaneRows}
}

func (m Model) layout() domain.RunViewLayout {
	return rules.ComputeRunViewLayout(rules.RunViewLayoutParams{
		Width:       m.width,
		Height:      m.height,
		NoticeLines: len(m.report()),
	})
}

func (m Model) applyJobs(msg jobsMsg) (Model, tea.Cmd) {
	m.jobs, m.err = msg.jobs, msg.err
	return m.setSelection(m.resolveSelection())
}

// resolveSelection keeps the cursor on the job it was on for as long as that
// job is both declared and shown, and falls back to the first one visible.
func (m Model) resolveSelection() string {
	visible := m.visible()
	for _, view := range visible {
		if view.Name == m.selected {
			return m.selected
		}
	}
	if len(visible) == 0 {
		return ""
	}
	return visible[0].Name
}

// fillSelectedPane binds the selected job to the one thing that can fill its
// pane: its live stream while it has one, the log file otherwise. Only the
// selected job is ever attached — a second subscription would double a job's
// output on screen and size its PTY behind the pane that is being read.
func (m Model) fillSelectedPane() (Model, tea.Cmd) {
	view, found := m.selectedView()
	if !found || m.pending == view.Name {
		return m, nil
	}

	// A job the run is starting is already writing into its pane through the
	// sequence; attaching would replay what it wrote and show it twice.
	if m.sequenceHolds(view.Name) {
		return m, nil
	}

	entry, held := m.panes.entry(view.Name)
	if view.Attachable {
		if held && entry.source == sourceLive && entry.stream != nil {
			return m, nil
		}
		m.pending = view.Name
		return m, m.attachCmd(attachParams{Job: view.Name, Size: m.paneSize()})
	}

	if held {
		return m, nil
	}
	m.pending = view.Name
	return m, m.historyCmd(view.Name)
}

func (m Model) kindOf(job string) domain.JobKind {
	for _, view := range m.jobs {
		if view.Name == job {
			return view.Kind
		}
	}
	return ""
}

// sequenceHolds reports that the run is writing into the job's pane itself.
// Those bytes are nowhere else yet: no subscription carries them and the log
// file is still being written, so the pane is the only copy.
func (m Model) sequenceHolds(job string) bool {
	return m.sequence.active && m.sequence.job == job
}

func (m Model) applyAttached(msg attachedMsg) (Model, tea.Cmd) {
	if m.pending == msg.job {
		m.pending = ""
	}
	// A stream that arrives for a job the cursor has left is closed on the spot:
	// nothing will read it, and an unread subscription holds the job's output
	// queue and its PTY size.
	if msg.job != m.selected {
		if msg.stream != nil {
			msg.stream.Close()
		}
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}

	m.err = nil
	pane := m.panes.attach(attachPaneParams{Job: msg.job, Stream: msg.stream})
	go readStream(readParams{
		Job:          msg.job,
		Stream:       msg.stream,
		Pane:         pane,
		Msgs:         m.msgs,
		Done:         m.runCtx.Done(),
		NormalizeEOL: rules.RunsOnPipe(m.kindOf(msg.job)),
	})

	model, tick := m.startTicking()
	// The window may have moved while the daemon was answering, and the size the
	// subscription carried is the one it opened with.
	if size := pane.Size(); size != msg.size {
		return model, tea.Batch(tick, resizeCmd(resizeParams{Streams: []runlogs.Stream{msg.stream}, Size: size}))
	}
	return model, tick
}

// applyStreamEnded is the job's output running out. The pane keeps what it
// printed, the subscription goes, and so does the keyboard: there is nothing
// left to type into.
func (m Model) applyStreamEnded(msg streamEndedMsg) (Model, tea.Cmd) {
	m.panes.endStream(msg.job)
	if m.pending == msg.job {
		m.pending = ""
	}
	if m.selected == msg.job {
		m.focused = false
	}
	return m, tea.Batch(m.refreshCmd(), m.listenCmd())
}

func (m Model) applyHistory(msg historyMsg) (Model, tea.Cmd) {
	if m.pending == msg.job {
		m.pending = ""
	}
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.err = nil
	m.panes.writeLines(writeLinesParams{Job: msg.job, Lines: msg.lines})
	return m, nil
}

// startTicking schedules the redraw clock unless it is already running: a pane
// is written to as the bytes arrive and drawn at domain.RunViewRenderFPS, which
// is only worth a tick while something is feeding one.
func (m Model) startTicking() (Model, tea.Cmd) {
	if m.ticking {
		return m, nil
	}
	m.ticking = true
	return m, frameCmd()
}

type writeParams struct {
	Job    string
	Stream runlogs.Stream
	Bytes  []byte
}

// writeCmd feeds a job's stdin off the render loop: the bytes travel to the
// daemon, and a keystroke is not worth blocking a frame on.
func writeCmd(params writeParams) tea.Cmd {
	return func() tea.Msg {
		if err := params.Stream.Write(params.Bytes); err != nil {
			return streamErrMsg{job: params.Job, err: err}
		}
		return nil
	}
}

type resizeParams struct {
	Streams []runlogs.Stream
	Size    PaneSize
}

// resizeCmd sizes the PTYs behind the panes that moved. A refusal is not worth
// reporting: a stream closed between the measurement and the round-trip is a
// pane nobody is looking at any more.
func resizeCmd(params resizeParams) tea.Cmd {
	if len(params.Streams) == 0 {
		return nil
	}
	return func() tea.Msg {
		for _, stream := range params.Streams {
			stream.Resize(runlogs.Size{Cols: params.Size.Cols, Rows: params.Size.Rows})
		}
		return nil
	}
}

type readParams struct {
	Job    string
	Stream runlogs.Stream
	Pane   *Pane
	Msgs   chan<- tea.Msg
	// NormalizeEOL terminates the job's bare LFs the way a PTY would, for a job
	// whose output reaches here off a pipe instead.
	NormalizeEOL bool
	// Done is the view being gone, which is the only thing that lets a reader
	// stop waiting for its last message to be taken.
	Done <-chan struct{}
}

// readStream drains a job's output into its pane as it arrives. It draws
// nothing: the model's tick is what paces the screen, so a job printing
// megabytes costs writes into the emulator rather than renders of it.
func readStream(params readParams) {
	pendingCR := false
	for chunk := range params.Stream.Chunks() {
		if params.NormalizeEOL {
			normalized := rules.NormalizeEOL(rules.NormalizeEOLParams{Chunk: chunk, PendingCR: pendingCR})
			chunk, pendingCR = normalized.Chunk, normalized.PendingCR
		}
		params.Pane.Write(chunk)
	}
	// Nothing else reports the end of a stream — the poll only re-reads the job
	// list — so dropping it on a busy model would leave the subscription
	// registered for the life of the view: a dead pane redrawn at 30 fps, and a
	// focus that writes keystrokes into a closed stream.
	select {
	case params.Msgs <- streamEndedMsg{job: params.Job}:
	case <-params.Done:
	}
}

func (m Model) visible() []runlogs.JobView {
	views := make([]runlogs.JobView, 0, len(m.jobs))
	for _, view := range m.jobs {
		if rules.MatchesJobFilter(view.Name, m.filter) {
			views = append(views, view)
		}
	}
	return views
}

func (m Model) selectedView() (runlogs.JobView, bool) {
	for _, view := range m.jobs {
		if view.Name == m.selected {
			return view, true
		}
	}
	return runlogs.JobView{}, false
}

func (m Model) selectedIndex() int {
	for index, view := range m.visible() {
		if view.Name == m.selected {
			return index
		}
	}
	return 0
}
