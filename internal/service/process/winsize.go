package process

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

type winsizeParams struct {
	File *os.File
	Cols int
	Rows int
}

// setWinsize is the package's only TIOCSWINSZ: a local terminal mirroring its
// own size onto the PTY it attached to, and a remote pane asking for the size
// it renders the job at, both land here.
func setWinsize(params winsizeParams) error {
	ws := winsize{Rows: uint16(params.Rows), Cols: uint16(params.Cols)}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		params.File.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
