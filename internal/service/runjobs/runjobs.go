// Package runjobs reads the run daemon's index of jobs. It exists so a surface
// that is not a cobra command — the dashboard — can ask the same question the
// CLI asks, without reaching into internal/commands.
package runjobs

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// List fetches the daemon's jobs. A daemon exits once no foreground job is left,
// so nobody listening says nothing about whether detached stacks are up: when
// the index still holds some, one is started to read them back. When it holds
// nothing there is nothing to report, and no daemon is forked for it.
//
// The error worth surfacing is a daemon of another build: reported as "no jobs",
// it would be the exact silence the version handshake exists to break.
func List() ([]domain.JobInfo, error) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		if !process.HasAnyIndexedJob() {
			return nil, nil
		}
		global, err := config.LoadGlobal()
		if err != nil {
			return nil, nil
		}
		if err := process.EnsureDaemon(process.DaemonParams{
			SocketPath: socketPath,
			ProxyPort:  rules.ProxyPort(global),
		}); err != nil {
			return nil, nil
		}
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		if errors.Is(err, domain.ErrDaemonVersionMismatch) {
			return nil, err
		}
		return nil, nil
	}
	return resp.Jobs, nil
}

// Load is List for a caller whose answer to "no jobs" and to "could not ask" is
// the same: nothing is running.
func Load() []domain.JobInfo {
	jobs, _ := List()
	return jobs
}

// Peek is Load for a reader whose question does not justify waking anything: it
// reports what a live daemon holds, and nothing when none is listening. The
// dashboard's poll reads through it — forking a daemon every three seconds is
// not what a background refresh is for.
func Peek() []domain.JobInfo {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return nil
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil
	}
	return resp.Jobs
}
