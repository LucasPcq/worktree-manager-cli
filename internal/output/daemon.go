package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func WriteDaemonStatusJSON(w io.Writer, status domain.DaemonStatus) error {
	return encodeJSON(w, status)
}

func DaemonStatusReport(w io.Writer, status domain.DaemonStatus) {
	lines := []string{
		fmt.Sprintf(domain.DaemonStatusStateFmt, daemonState(status)),
		fmt.Sprintf(domain.DaemonStatusVersFmt, daemonVersion(status)),
	}
	if status.Running {
		lines = append(lines, fmt.Sprintf(domain.DaemonStatusJobsFmt, status.Foreground, status.Detached))
		if status.ProxyPort > 0 {
			lines = append(lines, fmt.Sprintf(domain.DaemonStatusProxyFmt, status.ProxyPort))
		}
	}
	lines = append(lines,
		fmt.Sprintf(domain.DaemonStatusSocketFmt, status.SocketPath),
		fmt.Sprintf(domain.DaemonStatusIndexFmt, status.StatePath),
	)
	Section(w, domain.DaemonStatusTitle, lines)

	if rules.DaemonVersionDiverged(status) {
		Callout(w, domain.DaemonMismatchTitle, rules.DaemonVersionMismatchLines(rules.DaemonVersionMismatchParams{
			Client: status.Version,
			Daemon: status.DaemonVersion,
		}))
	}
}

func daemonState(status domain.DaemonStatus) string {
	if !status.Running {
		return domain.DaemonStatusStopped
	}
	return fmt.Sprintf(domain.DaemonStatusUpFmt, status.PID)
}

func daemonVersion(status domain.DaemonStatus) string {
	if !status.Running || status.DaemonVersion == status.Version {
		return status.Version
	}
	return fmt.Sprintf("%s (daemon: %s)", status.Version, rules.DaemonVersionLabel(status.DaemonVersion))
}
