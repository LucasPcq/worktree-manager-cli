package process

import "unsafe"

type winsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

func unsafeWinsize(width int, height int) unsafe.Pointer {
	ws := &winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	}
	return unsafe.Pointer(ws)
}
