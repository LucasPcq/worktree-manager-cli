// Package runview renders a job's raw PTY output through a terminal emulator.
package runview

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PaneSize struct {
	Cols int
	Rows int
}

type PaneParams struct {
	Size           PaneSize
	ScrollbackSize int
}

// Pane serializes access to the emulator itself: vt.SafeEmulator guards only
// part of what it exposes — WriteString, String, Close, Bounds and InputPipe
// are promoted unlocked from the embedded *Emulator.
type Pane struct {
	mu     sync.Mutex
	term   *vt.Emulator
	size   PaneSize
	offset int
}

func NewPane(params PaneParams) *Pane {
	size := resolvePaneSize(params.Size)
	term := vt.NewEmulator(size.Cols, size.Rows)
	term.SetScrollbackSize(orDefault(params.ScrollbackSize, domain.JobPaneScrollbackLines))
	return &Pane{term: term, size: size}
}

func (p *Pane) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	before := p.term.ScrollbackLen()
	n, err := p.term.Write(b)
	if p.offset > 0 {
		// Scrolled back, the offset is an anchor into the history: follow the
		// lines the reader is looking at as new output pushes them up.
		p.offset += p.term.ScrollbackLen() - before
		p.clampLocked()
	}
	return n, err
}

// Resize reports whether the size actually changed. x/vt never reflows, so only
// the job's own process can redraw at the new size and the caller has to resize
// its PTY too — a round-trip worth skipping when nothing moved.
func (p *Pane) Resize(size PaneSize) (changed bool) {
	next := resolvePaneSize(size)
	p.mu.Lock()
	defer p.mu.Unlock()
	if next == p.size {
		return false
	}
	p.term.Resize(next.Cols, next.Rows)
	p.size = next
	p.clampLocked()
	return true
}

func (p *Pane) Size() PaneSize {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size
}

func (p *Pane) ScrollUp(lines int) { p.scrollBy(lines) }

func (p *Pane) ScrollDown(lines int) { p.scrollBy(-lines) }

func (p *Pane) ScrollToLive() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset = 0
}

// ScrollOffset is the number of scrollback lines above the live tail; zero
// means the pane follows the job's output.
func (p *Pane) ScrollOffset() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.offset
}

// Render returns the visible rows with their SGR sequences, ready to drop into
// a lipgloss view.
func (p *Pane) Render() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offset == 0 {
		return p.term.Render()
	}

	scrollback := p.term.Scrollback()
	lines := make([]string, 0, p.size.Rows)
	for i := p.offset; i > 0 && len(lines) < p.size.Rows; i-- {
		line := scrollback.Line(scrollback.Len() - i).Render()
		lines = append(lines, ansi.Truncate(line, p.size.Cols, ""))
	}
	if len(lines) == p.size.Rows {
		return strings.Join(lines, "\n")
	}

	for line := range strings.SplitSeq(p.term.Render(), "\n") {
		if len(lines) == p.size.Rows {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (p *Pane) scrollBy(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset += lines
	p.clampLocked()
}

func (p *Pane) clampLocked() {
	p.offset = min(max(p.offset, 0), p.term.ScrollbackLen())
}

func resolvePaneSize(size PaneSize) PaneSize {
	return PaneSize{
		Cols: orDefault(size.Cols, domain.JobPaneDefaultCols),
		Rows: orDefault(size.Rows, domain.JobPaneDefaultRows),
	}
}

func orDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
