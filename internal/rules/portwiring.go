package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobsMissingPortRefParams struct {
	Config domain.RunConfig
	// Exempt names the jobs whose ports are read somewhere other than their
	// command — a compose stack takes them from its file.
	Exempt []string
}

// JobsMissingPortRef names the services declaring a port their command never
// mentions. It proves nothing: a command can read the variable from the
// environment on its own. It only asks the question wtm cannot answer, at the
// one moment the answer is still cheap.
func JobsMissingPortRef(params JobsMissingPortRefParams) []domain.JobCmdFix {
	exempt := make(map[string]bool, len(params.Exempt))
	for _, name := range params.Exempt {
		exempt[name] = true
	}

	var fixes []domain.JobCmdFix
	for _, job := range params.Config.Jobs {
		if !ShouldProbeJob(job.Kind, job.Ports) || exempt[job.Name] {
			continue
		}
		var missing []string
		for _, name := range sortedPortNames(job.Ports) {
			if !strings.Contains(job.Cmd, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			fixes = append(fixes, domain.JobCmdFix{Job: job.Name, Cmd: job.Cmd, Vars: missing})
		}
	}
	sort.SliceStable(fixes, func(i, j int) bool { return fixes[i].Job < fixes[j].Job })
	return fixes
}

type PortIsolationLinesParams struct {
	Unported []string
	Ignoring []domain.JobCmdFix
}

// PortIsolationLines names every job that will bind the same port in every
// worktree, and why. A job with no port and a job whose command ignores the one
// it was given fail the same way, so they are listed together.
func PortIsolationLines(params PortIsolationLinesParams) []string {
	if len(params.Unported) == 0 && len(params.Ignoring) == 0 {
		return nil
	}

	width := 0
	for _, job := range params.Unported {
		width = max(width, len([]rune(job)))
	}
	for _, fix := range params.Ignoring {
		width = max(width, len([]rune(fix.Job)))
	}

	var lines []string
	for _, job := range params.Unported {
		lines = append(lines, fmt.Sprintf(domain.PortIsolationLineFmt, pad(job, width), domain.PortIsolationNoPort))
	}
	for _, fix := range params.Ignoring {
		lines = append(lines, fmt.Sprintf(domain.PortIsolationLineFmt, pad(fix.Job, width),
			fmt.Sprintf(domain.PortIsolationIgnoresFmt, strings.Join(fix.Vars, ", "))))
	}
	return append(lines, "", domain.PortIsolationHint)
}
