package process

import "os"

// terminateParams holds inputs for terminating a running job's process tree.
type terminateParams struct {
	Process *os.Process
	Name    string
}
