package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ShouldProbeJob singles out the jobs a probe has anything to say about: a
// service that declared a port. A task does not listen, and a service with no
// declaration has nothing to check.
func ShouldProbeJob(kind domain.JobKind, ports map[string]int) bool {
	return kind == domain.JobKindService && len(ports) > 0
}

type DiagnosePortProbesParams struct {
	Job string
	// Resolved is the ports the daemon reported binding, base plus offset.
	Resolved map[string]int
	// Listening is what answered, keyed by port. It carries the base ports too,
	// so a silent port can be told apart from one whose variable never arrived.
	Listening map[int]bool
	// Offset is this worktree's port offset. At zero the base and the resolved
	// port are the same, which is what makes the BaseListening hint meaningless
	// on the main checkout.
	Offset int
}

// DiagnosePortProbes turns what was observed into one verdict per declared
// port, in a stable order.
func DiagnosePortProbes(params DiagnosePortProbesParams) []domain.PortProbe {
	names := make([]string, 0, len(params.Resolved))
	for name := range params.Resolved {
		names = append(names, name)
	}
	sort.Strings(names)

	probes := make([]domain.PortProbe, 0, len(names))
	for _, name := range names {
		port := params.Resolved[name]
		probe := domain.PortProbe{Job: params.Job, Name: name, Port: port, Status: domain.PortSilent}

		switch {
		case params.Listening[port]:
			probe.Status = domain.PortListening
		case params.Offset != 0 && params.Listening[port-params.Offset]:
			probe.BaseListening = port - params.Offset
		}
		probes = append(probes, probe)
	}
	return probes
}

// PortsToDial is every port a probe must ask about: each resolved port, plus
// the base it shifted from, so one pass gathers what the diagnosis needs.
func PortsToDial(params DiagnosePortProbesParams) []int {
	seen := map[int]bool{}
	var ports []int
	add := func(p int) {
		if p <= 0 || seen[p] {
			return
		}
		seen[p] = true
		ports = append(ports, p)
	}

	for _, port := range params.Resolved {
		add(port)
		if params.Offset != 0 {
			add(port - params.Offset)
		}
	}
	sort.Ints(ports)
	return ports
}

// PortProbeLines reports only what went wrong. A port that answers is the
// expected case and the run already said the job started.
func PortProbeLines(probes []domain.PortProbe) []string {
	var lines []string
	for _, p := range probes {
		if p.Status == domain.PortListening {
			continue
		}
		lines = append(lines, fmt.Sprintf(domain.PortProbeSilentFmt, p.Job, p.Name, p.Port))
		if p.BaseListening > 0 {
			lines = append(lines,
				fmt.Sprintf(domain.PortProbeBaseFmt, p.BaseListening),
				domain.PortProbeBaseHint)
		}
	}
	return lines
}

// AllPortsListening reports whether every probe answered, which is what lets a
// poll stop early instead of spending its whole budget on a healthy stack.
func AllPortsListening(probes []domain.PortProbe) bool {
	for _, p := range probes {
		if p.Status != domain.PortListening {
			return false
		}
	}
	return true
}

// PortProbeBudget resolves how long the check may wait. Zero means "unset", so
// it takes the default; a negative value is an explicit refusal and disables
// the check, which is what makes `port_probe_timeout = -1` a way to turn it off
// for a whole repo.
func PortProbeBudget(cfg domain.RunConfig) time.Duration {
	switch {
	case cfg.PortProbeTimeout == 0:
		return domain.PortProbeTimeout
	case cfg.PortProbeTimeout < 0:
		return 0
	default:
		return time.Duration(cfg.PortProbeTimeout) * time.Second
	}
}

// PortOffsetFromEnv reads the offset the surface resolved. An unreadable value
// degrades to zero: a probe that cannot tell base from resolved withholds the
// hint rather than pointing at the wrong port.
func PortOffsetFromEnv(env map[string]string) int {
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return 0
	}
	return offset
}

// DedupePorts keeps one dial per port across every job, in a stable order.
func DedupePorts(ports []int) []int {
	seen := make(map[int]bool, len(ports))
	unique := make([]int, 0, len(ports))
	for _, port := range ports {
		if seen[port] {
			continue
		}
		seen[port] = true
		unique = append(unique, port)
	}
	sort.Ints(unique)
	return unique
}

// ServicesWithoutPorts names the services no port was declared for, in a stable
// order. They run, but nothing shifts them per worktree, and the probe has
// nothing to check where nothing is declared.
func ServicesWithoutPorts(cfg domain.RunConfig) []string {
	var jobs []string
	for _, job := range cfg.Jobs {
		if job.Kind == domain.JobKindService && len(job.Ports) == 0 {
			jobs = append(jobs, job.Name)
		}
	}
	sort.Strings(jobs)
	return jobs
}

// PortEntriesFor is every service a surface can review, in a stable order: the
// jobs as declared, their ports by name. A service detection found no port for
// gets a row too, with no base — it is the one the user alone can settle, and
// leaving it out is what let an init end on a config that only starts on main.
type PortEntriesForParams struct {
	Config domain.RunConfig
	// ComposeJobs are the jobs whose ports come from a compose file's `ports:`
	// list. That list is the whole story, so nothing more is offered for them.
	ComposeJobs []string
}

func PortEntriesFor(params PortEntriesForParams) []domain.PortEntry {
	fromCompose := make(map[string]bool, len(params.ComposeJobs))
	for _, name := range params.ComposeJobs {
		fromCompose[name] = true
	}
	taken := map[string]bool{}
	for _, job := range params.Config.Jobs {
		for name := range job.Ports {
			taken[name] = true
		}
	}

	var entries []domain.PortEntry
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindService {
			continue
		}
		for _, name := range sortedPortNames(job.Ports) {
			entries = append(entries, domain.PortEntry{Job: job.Name, Name: name, Base: job.Ports[name]})
		}
		if fromCompose[job.Name] || declaresAListeningPort(job) {
			continue
		}
		name := freePortName(job.Name, taken)
		taken[name] = true
		entries = append(entries, domain.PortEntry{Job: job.Name, Name: name})
	}
	return entries
}

// declaresAListeningPort reads the one direction the .env convention carries: a
// key named exactly PORT is the port a process binds, while a *_PORT is as
// likely to be something it dials (DB_PORT, REDIS_PORT). A job holding only the
// latter has not said what it listens on, and wtm must not conclude for it.
func declaresAListeningPort(job domain.JobConfig) bool {
	for name := range job.Ports {
		if name == domain.PortNameDefault {
			return true
		}
	}
	return false
}

// freePortName proposes PORT, falling back to one derived from the job when
// another job already carries it. Two jobs may each hold their own PORT — their
// environments are separate — but the env-file linking flattens variables to one
// base, so a name proposed twice would make a .env key follow the wrong port.
func freePortName(job string, taken map[string]bool) string {
	if !taken[domain.PortNameDefault] {
		return domain.PortNameDefault
	}
	return EnvVarNameFor(job) + "_" + domain.PortNameDefault
}

// EnvVarNameFor turns a name into the shape an environment variable takes. The
// result is always a usable identifier: it is written into run.toml and injected
// into a process, and ValidateRunPorts refuses anything else — a name starting
// with a digit would turn an init into a config that cannot be loaded back.
func EnvVarNameFor(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	varName := strings.Trim(collapseUnderscores(b.String()), "_")
	if varName == "" {
		return domain.ComposeVarNamePrefix
	}
	if varName[0] >= '0' && varName[0] <= '9' {
		return domain.ComposeVarNamePrefix + varName
	}
	return varName
}
