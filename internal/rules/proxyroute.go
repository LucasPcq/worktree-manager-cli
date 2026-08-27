package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RouteHostParams struct {
	Job domain.JobConfig
	// Worktree and Project are the raw names; they are made DNS-safe here so a
	// caller never has to remember to.
	Worktree string
	Project  string
}

// RouteHost is the hostname a job is published under, empty for one that
// publishes nothing.
//
// The order is <job>.<worktree>.<project> and not the reverse: a cookie set on
// .<worktree>.<project>.localhost is then shared by that worktree's jobs and
// invisible to the others, which is the whole point. The reverse order would
// leak a cookie from one worktree to the next.
func RouteHost(params RouteHostParams) string {
	label := JobHostLabel(params.Job)
	if label == "" {
		return ""
	}
	return strings.Join([]string{
		label,
		HostLabel(params.Worktree),
		HostLabel(params.Project),
		domain.ProxyTLD,
	}, ".")
}
