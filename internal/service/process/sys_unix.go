//go:build !windows

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// SupportedOnPlatform reports whether the run module can operate on this OS.
func SupportedOnPlatform() error {
	return nil
}

func notifyResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}

func setOwnProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// terminateTree signals the whole process group with a negative PID, so npm AND
// every node child it spawned receive SIGTERM. ESRCH means the group is already
// gone, which is fine. Any other failure (e.g. job spawned without its own
// group) falls back to signalling just the parent.
func terminateTree(params terminateParams) error {
	err := syscall.Kill(-params.Process.Pid, syscall.SIGTERM)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}

	if sigErr := params.Process.Signal(syscall.SIGTERM); sigErr != nil && !errors.Is(sigErr, os.ErrProcessDone) {
		return fmt.Errorf("signal %s: %w", params.Name, sigErr)
	}
	return nil
}

func killTree(process *os.Process) {
	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
}
