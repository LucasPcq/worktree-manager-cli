//go:build windows

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/LucasPcq/wtm/internal/domain"
)

// detachedProcess combines DETACHED_PROCESS (0x8, no inherited console) with
// CREATE_NEW_PROCESS_GROUP so the child outlives the launching terminal.
// Only CREATE_NEW_PROCESS_GROUP is exported by the syscall package.
const detachedProcess = 0x00000008 | syscall.CREATE_NEW_PROCESS_GROUP

// SupportedOnPlatform reports whether the run module can operate on this OS.
// Windows has no PTY that creack/pty can drive, so services cannot be hosted.
func SupportedOnPlatform() error {
	return domain.ErrRunUnsupportedPlatform
}

func notifyResize(chan<- os.Signal) {}

func setOwnProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}

// terminateTree has no graceful equivalent on Windows: there is no signal to
// send a console process group that a child is guaranteed to honour, so the
// process is killed outright.
func terminateTree(params terminateParams) error {
	if err := params.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill %s: %w", params.Name, err)
	}
	return nil
}

func killTree(process *os.Process) {
	_ = process.Kill()
}
