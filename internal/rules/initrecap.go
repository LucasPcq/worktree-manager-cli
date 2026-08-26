package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RecapJobLines is one line per job: its name, then the ports it will carry.
// The recap exists so the run can be judged before it is written, and a count
// ("3 ports") cannot be judged — the numbers can.
func RecapJobLines(cfg domain.RunConfig) []string {
	width := 0
	for _, job := range cfg.Jobs {
		width = max(width, len([]rune(job.Name)))
	}

	lines := make([]string, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		lines = append(lines, fmt.Sprintf(domain.RecapJobLineFmt, pad(job.Name, width), recapPorts(job)))
	}
	return lines
}

func recapPorts(job domain.JobConfig) string {
	if job.Kind != domain.JobKindService {
		return domain.RecapTask
	}
	if len(job.Ports) == 0 {
		return domain.RecapNoPort
	}

	parts := make([]string, 0, len(job.Ports))
	for _, name := range sortedPortNames(job.Ports) {
		parts = append(parts, fmt.Sprintf(domain.RecapPortFmt, name, job.Ports[name]))
	}
	return strings.Join(parts, domain.RecapPortSep)
}

// RecapProfileLines is one line per profile: what `run up <name>` will start.
func RecapProfileLines(cfg domain.RunConfig) []string {
	width := 0
	for _, profile := range cfg.Profiles {
		width = max(width, len([]rune(profile.Name)))
	}

	lines := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		line := fmt.Sprintf(domain.RecapJobLineFmt, pad(profile.Name, width),
			strings.Join(profile.Jobs, domain.ProfileListJobSep))
		if profile.Default {
			line += domain.RecapDefaultSuffix
		}
		lines = append(lines, line)
	}
	return lines
}
