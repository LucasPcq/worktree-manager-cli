package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvSummary is the trailing verdict of `wtm env`: what to say, and whether it
// reads as an accomplishment or as a plain note.
type EnvSummary struct {
	Text string
	Done bool
}

// EnvOutcomeSummary tallies everything the run actually wrote. The port pass is
// counted alongside the files because it writes to the same .env: a run that
// shifted a port and changed nothing else has written changes, and saying "no
// changes written" there is not a wording problem, it is a false report.
func EnvOutcomeSummary(result domain.EnvSyncResult) EnvSummary {
	if result.Check {
		if result.HasDrift() {
			return EnvSummary{Text: domain.EnvCheckDriftMessage}
		}
		return EnvSummary{Text: domain.EnvCheckCleanMessage, Done: true}
	}

	files, ports := result.AppliedFiles(), 0
	if result.Ports.Applied {
		ports = len(result.Ports.Rewrites())
	}
	switch {
	case files == 0 && ports == 0:
		return EnvSummary{Text: domain.EnvNothingWrittenMessage}
	case ports == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledFmt, files), Done: true}
	case files == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvPortsShiftedFmt, ports), Done: true}
	default:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledAndShiftedFmt, files, ports), Done: true}
	}
}
