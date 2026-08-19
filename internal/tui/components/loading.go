package components

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/styles"
)

// renderLoadingBox renders an animated spinner glyph + muted message inside the
// shared bordered status box. Used by both the standalone loader and the wizard's
// async status banner so the loading look stays identical.
func renderLoadingBox(spinnerView, message string) string {
	return styles.StatusBox.Render(spinnerView + " " + styles.Muted.Render(message))
}

// newMutedSpinner returns the project's standard loading spinner: a MiniDot
// braille animation in the muted style.
func newMutedSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = styles.Muted
	return sp
}

// MutedSpinner is the project's standard spinner, for surfaces that animate
// their own wait rather than going through RunLoading.
func MutedSpinner() spinner.Model { return newMutedSpinner() }

// LoadingParams configures RunLoading.
type LoadingParams struct {
	// Message is the text shown next to the spinner.
	Message string
	// Work is the blocking operation to run while the loader animates. Capture any
	// results via closure.
	Work func() error
	// Animate gates the visual loader: when false (e.g. JSON output), Work runs
	// directly with no box. The box is also skipped when stderr is not a terminal.
	Animate bool
}

type loadingDoneMsg struct{ err error }

// loadingModel shows a bordered, animated loading box while Work runs in the
// background, then quits. Its View returns "" once done so the box is cleared
// from the terminal, leaving the command's framed result as the only output.
type loadingModel struct {
	spinner spinner.Model
	message string
	work    tea.Cmd
	err     error
	done    bool
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.work)
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadingDoneMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	// Ignore key presses: the blocking work is not cancellable.
	return m, nil
}

func (m loadingModel) View() string {
	if m.done {
		return ""
	}
	return "\n" + renderLoadingBox(m.spinner.View(), m.message) + "\n"
}

// RunLoading runs work while showing an animated bordered loading box on stderr,
// then clears the box and returns work's error. When params.Animate is false or
// stderr is not a terminal, work runs directly with no box, so piped and JSON
// output stay clean.
func RunLoading(params LoadingParams) error {
	if !params.Animate || !term.IsTerminal(int(os.Stderr.Fd())) {
		return params.Work()
	}

	m := loadingModel{
		spinner: newMutedSpinner(),
		message: params.Message,
		work: func() tea.Msg {
			return loadingDoneMsg{err: params.Work()}
		},
	}

	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if lm, ok := final.(loadingModel); ok {
		return lm.err
	}
	return nil
}
