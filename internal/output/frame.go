package output

import "io"

// Frame renders human-facing command output with the canonical vertical padding:
// exactly one blank line before the body and one after. Use it for any command
// whose entire human output can be produced inside render.
//
// JSON and machine-readable (shell-eval) paths must NOT be framed — they emit
// flush output and never call Frame/FrameStart/FrameEnd.
func Frame(w io.Writer, render func()) {
	FrameStart(w)
	render()
	FrameEnd(w)
}

// FrameStart writes the single leading blank line of a command frame. Pair it
// with exactly one FrameEnd. Prefer Frame; use the explicit pair only for
// streaming, spinner-driven, or split-stream commands whose body cannot be a
// single closure — e.g. content that starts on stderr (a plan or spinner) and
// finishes on stdout (the result), or output interleaved with live spinners.
func FrameStart(w io.Writer) {
	Blank(w)
}

// FrameEnd writes the single trailing blank line of a command frame. For
// split-stream commands it may be called on a different writer than FrameStart
// (e.g. FrameStart(stderr) … FrameEnd(stdout)).
func FrameEnd(w io.Writer) {
	Blank(w)
}
