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
//
// The ioctl goes through SyscallConn().Control, which holds a reference on the
// descriptor for its duration and refuses once the file is closed. File.Fd()
// takes no such reference: it reads a raw number that a concurrent Close may
// already have handed back to the OS — and the daemon recycles descriptors
// constantly (PTYs, log files, accepted sockets), so sizing one job through a
// stale number silently resizes whatever job now owns it.
func setWinsize(params winsizeParams) error {
	ws := winsize{Rows: uint16(params.Rows), Cols: uint16(params.Cols)}

	conn, err := params.File.SyscallConn()
	if err != nil {
		return err
	}

	var errno syscall.Errno
	if err := conn.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(
			syscall.SYS_IOCTL,
			fd,
			syscall.TIOCSWINSZ,
			uintptr(unsafe.Pointer(&ws)),
		)
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}
