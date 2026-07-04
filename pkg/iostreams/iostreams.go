package iostreams

import (
	"bytes"
	"io"
	"os"

	"golang.org/x/term"
)


type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	stdinIsTTY  bool
	stdoutIsTTY bool
	stderrIsTTY bool

	neverPrompt bool
}

func System() *IOStreams {
	return &IOStreams{
		In:          os.Stdin,
		Out:         os.Stdout,
		ErrOut:      os.Stderr,
		stdinIsTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		stdoutIsTTY: term.IsTerminal(int(os.Stdout.Fd())),
		stderrIsTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{In: &bytes.Buffer{}, Out: out, ErrOut: errOut}, out, errOut
}

func TestInteractive() (*IOStreams, *bytes.Buffer, *bytes.Buffer) {
	io, out, errOut := Test()
	io.stdinIsTTY = true
	io.stdoutIsTTY = true
	io.stderrIsTTY = true
	return io, out, errOut
}

func (s *IOStreams) SetNeverPrompt(v bool) { s.neverPrompt = v }

func (s *IOStreams) CanPrompt() bool {
	return !s.neverPrompt && s.stdinIsTTY
}

func (s *IOStreams) ErrIsTTY() bool { return s.stderrIsTTY }
