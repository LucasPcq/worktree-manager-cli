// Package urls is where every address the run module hands out is computed
// from. It never contacts the daemon: a job's address is a property of its
// worktree's offset, known whether or not anything is running.
package urls

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Params struct {
	Context flow.Context
	Config  domain.RunConfig
	// Raw asks for the job's own port instead of the name the proxy serves it
	// under — the address the .env answers on whatever the addressing says.
	Raw bool
}

// Reader answers where each job is reachable, worktree by worktree.
type Reader struct {
	ctx       flow.Context
	config    domain.RunConfig
	proxyPort int
}

func Open(params Params) Reader {
	proxyPort := 0
	if !params.Raw {
		proxyPort = process.PublicProxyPort(rules.ProxyPort(params.Context.Config.Global))
	}
	return Reader{ctx: params.Context, config: params.Config, proxyPort: proxyPort}
}

// Serving reports whether a name is published at all: with no proxy, or under
// --raw, every address this hands out is a plain port.
func (r Reader) Serving() bool { return r.proxyPort > 0 }

// In lists the jobs reachable in one worktree. The worktree is what makes the
// addresses differ: its ordinal decides every port.
func (r Reader) In(dir string) []domain.JobURLEntry {
	env := seam.JobEnv(seam.JobEnvParams{ProjectDir: r.ctx.ProjectDir, StateDir: r.ctx.StateDir, WorkDir: dir})
	offset, _ := strconv.Atoi(env[domain.EnvPortOffset])
	project := filepath.Base(r.ctx.ProjectDir)
	proxyPort := r.publicPortIn(dir)

	var entries []domain.JobURLEntry
	for _, job := range r.config.Jobs {
		ports := rules.JobPorts(rules.JobPortsParams{Ports: job.Ports, PortOffset: offset})
		url := rules.JobURL(rules.JobURLParams{
			Job:        job,
			Ports:      ports,
			Host:       rules.RouteHost(rules.RouteHostParams{Job: job, Worktree: env[domain.EnvWorktree], Project: project}),
			PublicPort: proxyPort,
		})
		if url == "" {
			continue
		}
		entries = append(entries, domain.JobURLEntry{Job: job.Name, URL: url})
	}
	return entries
}

// publicPortIn is zero for a worktree whose .env still spells its addresses as
// ports: the name is published, but the only entrance the app answers on is the
// port, and handing out a url that fails is worse than handing out a plain one.
func (r Reader) publicPortIn(dir string) int {
	if r.proxyPort == 0 {
		return 0
	}
	if addressing.Read(addressing.Params{Context: r.ctx, WorkDirs: []string{dir}}).PortAddressed[dir] {
		return 0
	}
	return r.proxyPort
}
