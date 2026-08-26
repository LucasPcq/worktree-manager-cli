package rules

import (
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
