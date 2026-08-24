package output

import (
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// UnportedJobsReport names the services the init kept without a port.
func UnportedJobsReport(w io.Writer, jobs []string) {
	if len(jobs) == 0 {
		return
	}
	Blank(w)
	Callout(w, domain.UnportedJobsTitle, jobs)
}
