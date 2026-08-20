package runview

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Params struct {
	Session runlogs.Session
	// Job is selected when the view opens; empty takes the first one.
	Job string
}

// Model is the run view's root Bubbletea model: a job list, the selected job's
// terminal emulator, and nothing else. What a job is, whether it can be
// attached to and what it has printed are runlogs' answers, never its own.
type Model struct {
	session runlogs.Session
	panes   *paneStore
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

	// pending is the job whose pane is being filled — an attach or a history
	// read in flight — so a second one is not started behind it.
	pending string
	// ticking reports whether a redraw tick is already scheduled: a pane being
	// written to is redrawn on a clock, never once per chunk.
	ticking bool
}

func New(params Params) Model {
	return Model{
		session:  params.Session,
		selected: params.Job,
		panes:    newPaneStore(PaneSize{}),
		msgs:     make(chan tea.Msg, domain.RunViewMsgBuffer),
	}
}

// Run opens the view on the alternate screen. Leaving it detaches: the jobs it
// was showing keep running.
func Run(params Params) error {
	model := New(params)
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		return fmt.Errorf("run view: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), pollCmd(), listenCmd(m.msgs))
}

type jobsMsg struct {
	jobs []runlogs.JobView
	err  error
}

type attachedMsg struct {
	job    string
	stream runlogs.Stream
	err    error
}

type historyMsg struct {
	job   string
	lines []string
	err   error
}

type streamEndedMsg struct{ job string }

type frameMsg struct{}

type pollMsg struct{}

func (m Model) refreshCmd() tea.Cmd {
	session := m.session
	return func() tea.Msg {
		if err := session.Refresh(); err != nil {
			return jobsMsg{jobs: session.Jobs(), err: err}
		}
		return jobsMsg{jobs: session.Jobs()}
	}
}

func (m Model) attachCmd(job string, size PaneSize) tea.Cmd {
	session := m.session
	return func() tea.Msg {
		stream, err := session.Attach(runlogs.AttachParams{
			Job:  job,
			Size: runlogs.Size{Cols: size.Cols, Rows: size.Rows},
		})
		return attachedMsg{job: job, stream: stream, err: err}
	}
}

func (m Model) historyCmd(job string) tea.Cmd {
	session := m.session
	return func() tea.Msg {
		lines, err := session.History(runlogs.HistoryParams{Job: job})
		return historyMsg{job: job, lines: lines, err: err}
	}
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.RunViewPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

func frameCmd() tea.Cmd {
	return tea.Tick(time.Second/domain.RunViewRenderFPS, func(time.Time) tea.Msg { return frameMsg{} })
}

func listenCmd(msgs <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-msgs }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.applySize(), nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case jobsMsg:
		return m.applyJobs(msg)

	case attachedMsg:
		return m.applyAttached(msg)

	case historyMsg:
		return m.applyHistory(msg)

	case streamEndedMsg:
		if m.pending == msg.job {
			m.pending = ""
		}
		return m, tea.Batch(m.refreshCmd(), listenCmd(m.msgs))

	case pollMsg:
		return m, tea.Batch(m.refreshCmd(), pollCmd())

	case frameMsg:
		if !m.panes.hasStream() {
			m.ticking = false
			return m, nil
		}
		return m, frameCmd()
	}

	return m, nil
}

// applySize gives the emulators the room the layout just measured. x/vt never
// reflows, so a pane that changed size has to be redrawn by the job itself.
func (m Model) applySize() Model {
	m.panes.resize(m.paneSize())
	return m
}

func (m Model) paneSize() PaneSize {
	layout := m.layout()
	return PaneSize{Cols: layout.PaneCols, Rows: layout.PaneRows}
}

func (m Model) layout() domain.RunViewLayout {
	return rules.ComputeRunViewLayout(rules.RunViewLayoutParams{Width: m.width, Height: m.height})
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

	entry, held := m.panes.entry(view.Name)
	if view.Attachable {
		if held && entry.source == sourceLive {
			return m, nil
		}
		m.pending = view.Name
		return m, m.attachCmd(view.Name, m.paneSize())
	}

	if held {
		return m, nil
	}
	m.pending = view.Name
	return m, m.historyCmd(view.Name)
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
	pane := m.panes.attach(msg.job, msg.stream)
	go readStream(readParams{Job: msg.job, Stream: msg.stream, Pane: pane, Msgs: m.msgs})
	return m.startTicking()
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
	m.panes.writeLines(msg.job, msg.lines)
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

type readParams struct {
	Job    string
	Stream runlogs.Stream
	Pane   *Pane
	Msgs   chan<- tea.Msg
}

// readStream drains a job's output into its pane as it arrives. It draws
// nothing: the model's tick is what paces the screen, so a job printing
// megabytes costs writes into the emulator rather than renders of it.
func readStream(params readParams) {
	for chunk := range params.Stream.Chunks() {
		params.Pane.Write(chunk)
	}
	// A full queue means the model is busy, not gone: the poll re-reads the job
	// list anyway, so the end of a stream is never worth blocking a reader on.
	select {
	case params.Msgs <- streamEndedMsg{job: params.Job}:
	default:
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
