package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposePortPlan is what a run of the detection decided, before anything is
// written: the declarations to add, the mappings to rewrite, and what it is
// leaving alone. Withheld is the reason the feature is honest — a port wtm
// cannot actually isolate is reported, never declared.
type ComposePortPlan struct {
	PortsByFile map[string]map[string]int
	Patches     map[string][]domain.ComposePortBinding
	Withheld    []domain.ComposePortBinding
	Unreadable  []domain.ComposeScan
}

type PlanComposePortsParams struct {
	Scans map[string]domain.ComposeScan
	Files []string
	// Patch says the rewrite was authorized — by the wizard step or by the flag.
	// Without it a frozen mapping is withheld rather than declared: declaring a
	// port Docker will not honour would announce an isolation that does not exist.
	Patch bool
}

// PlanComposePorts turns the scans of the selected compose files into the
// declarations run.toml should carry. It writes nothing and reads no disk.
func PlanComposePorts(params PlanComposePortsParams) ComposePortPlan {
	plan := ComposePortPlan{
		PortsByFile: map[string]map[string]int{},
		Patches:     map[string][]domain.ComposePortBinding{},
	}

	for _, file := range params.Files {
		scan, found := params.Scans[file]
		if !found {
			continue
		}
		if scan.Err != "" {
			plan.Unreadable = append(plan.Unreadable, scan)
			continue
		}

		for _, binding := range scan.Bindings {
			switch {
			case binding.Status == domain.ComposePortTemplated:
				declareComposePort(plan.PortsByFile, file, binding)
			case binding.Status == domain.ComposePortFrozen && params.Patch:
				declareComposePort(plan.PortsByFile, file, binding)
				plan.Patches[file] = append(plan.Patches[file], binding)
			default:
				plan.Withheld = append(plan.Withheld, binding)
			}
		}
	}

	return plan
}

// declareComposePort keeps the first declaration of a variable: the scanner
// already made names unique per file, so a second one can only be a duplicate.
func declareComposePort(ports map[string]map[string]int, file string, binding domain.ComposePortBinding) {
	if ports[file] == nil {
		ports[file] = map[string]int{}
	}
	if _, exists := ports[file][binding.Var]; !exists {
		ports[file][binding.Var] = binding.Base
	}
}

type BackfillDockerPortsParams struct {
	Config      domain.RunConfig
	PortsByFile map[string]map[string]int
}

type BackfillDockerPortsResult struct {
	Config domain.RunConfig
	// Added is what this call actually wrote, job by job. A variable the job
	// already declared is never in it — and never overwritten.
	Added map[string]map[string]int
}

// BackfillDockerPorts gives each compose-backed job the ports detected in the
// file it runs, matched on the same "-f <file> " fragment BuildDockerJobs
// emits. It is additive by construction: a job that already declares a variable
// keeps the value it declares, which is what makes re-running `run init` safe
// on a project configured before ports existed.
func BackfillDockerPorts(params BackfillDockerPortsParams) BackfillDockerPortsResult {
	result := BackfillDockerPortsResult{
		Config: params.Config,
		Added:  map[string]map[string]int{},
	}
	result.Config.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(result.Config.Jobs, params.Config.Jobs)

	for _, file := range SortedComposeFiles(params.PortsByFile) {
		ports := params.PortsByFile[file]
		needle := DockerComposeFileFlag(file)

		for i, job := range result.Config.Jobs {
			if !jobRunsComposeFile(job, needle) {
				continue
			}
			for _, name := range sortedPortNames(ports) {
				if _, declared := result.Config.Jobs[i].Ports[name]; declared {
					continue
				}
				// Cloned on first write: the slice copy above shares every job's
				// Ports map with the config the caller passed in.
				result.Config.Jobs[i].Ports = clonePorts(result.Config.Jobs[i].Ports)
				result.Config.Jobs[i].Ports[name] = ports[name]

				if result.Added[job.Name] == nil {
					result.Added[job.Name] = map[string]int{}
				}
				result.Added[job.Name][name] = ports[name]
			}
		}
	}

	return result
}

func jobRunsComposeFile(job domain.JobConfig, needle string) bool {
	return strings.Contains(job.Cmd, needle) || strings.Contains(job.Stop, needle)
}

type PruneCollidingPortsParams struct {
	Config domain.RunConfig
	// Detected names the declarations wtm just added, job by job. Only these are
	// ever dropped: a base the user wrote by hand outranks one wtm inferred.
	Detected map[string]map[string]int
}

// DroppedPort is a detected declaration withdrawn because it could not coexist
// with another, and the declaration it clashed with.
type DroppedPort struct {
	Port      PortDeclaration
	Against   PortDeclaration
	Worktrees int
}

type PruneCollidingPortsResult struct {
	Config  domain.RunConfig
	Dropped []DroppedPort
}

// PruneCollidingPorts withdraws the detected declarations that would make the
// config unloadable. Two bases whose gap is a multiple of the offset block bind
// the same port some worktrees apart, and ValidateRunPorts refuses the file for
// it — so the check runs here, before writing, where the port can still be
// named and explained instead of surfacing later as a rejected run.toml.
func PruneCollidingPorts(params PruneCollidingPortsParams) PruneCollidingPortsResult {
	result := PruneCollidingPortsResult{Config: params.Config}

	detected := func(d PortDeclaration) bool {
		_, ok := params.Detected[d.Job][d.Name]
		return ok
	}
	dropped := map[string]map[string]bool{}
	isDropped := func(d PortDeclaration) bool { return dropped[d.Job][d.Name] }
	drop := func(d, against PortDeclaration, worktrees int) {
		if dropped[d.Job] == nil {
			dropped[d.Job] = map[string]bool{}
		}
		dropped[d.Job][d.Name] = true
		result.Dropped = append(result.Dropped, DroppedPort{Port: d, Against: against, Worktrees: worktrees})
	}

	for _, c := range PortCollisions(params.Config) {
		if isDropped(c.A) || isDropped(c.B) {
			continue
		}
		switch {
		case detected(c.A) && detected(c.B):
			drop(c.A, c.B, c.Worktrees)
			drop(c.B, c.A, c.Worktrees)
		case detected(c.A):
			drop(c.A, c.B, c.Worktrees)
		case detected(c.B):
			drop(c.B, c.A, c.Worktrees)
		}
	}

	if len(result.Dropped) == 0 {
		return result
	}

	result.Config.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(result.Config.Jobs, params.Config.Jobs)
	for i, job := range result.Config.Jobs {
		if len(dropped[job.Name]) == 0 {
			continue
		}
		kept := map[string]int{}
		for name, base := range job.Ports {
			if !dropped[job.Name][name] {
				kept[name] = base
			}
		}
		if len(kept) == 0 {
			kept = nil
		}
		result.Config.Jobs[i].Ports = kept
	}

	return result
}

func clonePorts(ports map[string]int) map[string]int {
	clone := make(map[string]int, len(ports)+1)
	for name, base := range ports {
		clone[name] = base
	}
	return clone
}
