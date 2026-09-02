package run

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// TestClientRefusesADaemonOfAnotherBuild covers the failure that looks like
// success: the daemon runs the jobs, so an older one keeps applying its own
// behaviour while the new client prints what it believes.
func TestClientRefusesADaemonOfAnotherBuild(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{Version: "0.27.0"})

	_, err := process.NewClient(process.SocketPath()).Send(process.Request{Action: process.ActionList})

	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	for _, want := range []string{"0.27.0", domain.Version, "wtm run daemon restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}
}

// TestClientReadsAnUnstampedAnswerAsOlder covers the daemon predating the
// handshake itself: it cannot announce a version, and its silence is the
// divergence.
func TestClientReadsAnUnstampedAnswerAsOlder(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{Version: "none"})

	_, err := process.NewClient(process.SocketPath()).Send(process.Request{Action: process.ActionList})

	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	if !strings.Contains(err.Error(), domain.DaemonVersionUnknown) {
		t.Errorf("message %q does not stand in for the missing version", err)
	}
}

func TestClientAcceptsADaemonOfTheSameBuild(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{})

	if _, err := process.NewClient(process.SocketPath()).Send(process.Request{Action: process.ActionList}); err != nil {
		t.Fatalf("the nominal path must cost nothing: %v", err)
	}
}
