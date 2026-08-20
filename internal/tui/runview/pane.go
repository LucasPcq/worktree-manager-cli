// Package runview renders a job's raw PTY output through a terminal emulator.
package runview

import (
	"sync"

	"github.com/charmbracelet/x/vt"
)

type PaneSize struct {
	Cols int
	Rows int
}

type PaneParams struct {
	Size           PaneSize
	ScrollbackSize int
}

// Pane guards the emulator itself: vt.SafeEmulator locks only part of the
// surface it exposes (WriteString, String and Close are promoted unlocked).
type Pane struct {
	mu   sync.Mutex
	term *vt.Emulator
}

func NewPane(params PaneParams) *Pane {
	term := vt.NewEmulator(params.Size.Cols, params.Size.Rows)
	term.SetScrollbackSize(params.ScrollbackSize)
	return &Pane{term: term}
}

func (p *Pane) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.term.Write(b)
}

func (p *Pane) Resize(size PaneSize) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.term.Resize(size.Cols, size.Rows)
}

func (p *Pane) Render() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.term.Render()
}
