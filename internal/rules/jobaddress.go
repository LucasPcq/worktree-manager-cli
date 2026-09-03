package rules

import (
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeJobAddressesParams struct {
	Config domain.RunConfig
	// PortOffset is the worktree's own, what makes the same job bind a different
	// port in each one.
	PortOffset int
	// Worktree and Project are the labels the proxy routes a published job on.
	Worktree   string
	Project    string
	PublicPort int
}

// WorktreeJobAddresses is where every declared job answers in one worktree —
// what `run url` computes job by job, gathered in one reading for a surface
// listing them all.
func WorktreeJobAddresses(params WorktreeJobAddressesParams) map[string]domain.JobAddress {
	if len(params.Config.Jobs) == 0 {
		return nil
	}
	addresses := make(map[string]domain.JobAddress, len(params.Config.Jobs))
	for _, job := range params.Config.Jobs {
		ports := JobPorts(JobPortsParams{Ports: job.Ports, PortOffset: params.PortOffset})
		addresses[job.Name] = domain.JobAddress{
			Ports: sortedPortValues(ports),
			URL: JobURL(JobURLParams{
				Job:        job,
				Ports:      ports,
				Host:       RouteHost(RouteHostParams{Job: job, Worktree: params.Worktree, Project: params.Project}),
				PublicPort: params.PublicPort,
			}),
		}
	}
	return addresses
}

// A map range would permute the ports between two reads of the same config.
func sortedPortValues(ports map[string]int) []int {
	values := make([]int, 0, len(ports))
	for _, port := range ports {
		values = append(values, port)
	}
	sort.Ints(values)
	return values
}
