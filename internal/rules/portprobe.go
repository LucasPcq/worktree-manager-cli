package rules

import (
	"fmt"
	"sort"
	"strconv"
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
