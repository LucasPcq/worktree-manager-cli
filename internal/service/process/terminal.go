package process

import (
	"fmt"
	"io"
)

// resetSequence undoes terminal modes commonly left on by interactive PTY
// applications (Turbo TUI, vite, vim, dev servers with mouse-driven HMR):
//   - 1000/1002/1003/1006/1015 : disable every mouse tracking mode
//   - 1049                     : leave the alternate screen
//   - 25h                      : re-show the cursor
//   - 0m                       : reset SGR colors/attributes
//
// The daemon-side process is untouched; only the user's local terminal
// state is restored. Without this, after Ctrl+C detach, mouse moves keep
// echoing "<button>;<col>;<row>M" at the shell prompt, the cursor stays
// hidden, and any alt-screen left active makes typing render as garbage.
const resetSequence = "\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?1015l\x1b[?1049l\x1b[?25h\x1b[0m"

// ResetTerminalState writes resetSequence to w. Errors are ignored — best
// effort is enough; the worst case is the user noticing the same garbage
// they would have seen without the call.
func ResetTerminalState(w io.Writer) {
	_, _ = fmt.Fprint(w, resetSequence)
}
