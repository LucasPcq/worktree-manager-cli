package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobURLParams struct {
	Job domain.JobConfig
	// Ports is what the job actually bound in this worktree, base plus offset.
	Ports map[string]int
}

// JobURL is where a job is reachable, empty for one that publishes nothing. This
// is the single place a surface asks; the proxy changes what it answers, not who
// asks.
func JobURL(params JobURLParams) string {
	if params.Job.URL == nil {
		return ""
	}
	port, bound := params.Ports[params.Job.URL.Port]
	if !bound {
		return ""
	}
	return fmt.Sprintf(domain.DirectURLFmt, port)
}
