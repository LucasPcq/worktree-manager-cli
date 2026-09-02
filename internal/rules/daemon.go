package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DaemonVersionMismatchParams struct {
	Client string
	Daemon string
}

// DaemonVersionMismatch words the refusal. It names both sides because the
// symptom of an old daemon is silence — the command reports what the new client
// believes while the old daemon does something else — so the message has to say
// what is actually running, and how to end it.
func DaemonVersionMismatch(params DaemonVersionMismatchParams) string {
	return fmt.Sprintf(domain.DaemonVersionMismatchFmt, DaemonVersionLabel(params.Daemon), params.Client)
}

// DaemonVersionDiverged reports a daemon this binary did not build. `status` is
// the one command that says so instead of refusing: it exists to be run when
// something else already refused.
func DaemonVersionDiverged(status domain.DaemonStatus) bool {
	return status.Running && status.DaemonVersion != status.Version
}

// DaemonVersionLabel names a daemon version for display, standing in for the
// build too old to stamp one.
func DaemonVersionLabel(version string) string {
	if version == "" {
		return domain.DaemonVersionUnknown
	}
	return version
}
