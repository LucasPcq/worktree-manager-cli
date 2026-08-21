package rules

import (
	"github.com/LucasPcq/wtm/internal/domain"
)

type ResolveComposePortsParams struct {
	Answers        domain.InitProjectAnswers
	PackageManager domain.PackageManager
	Existing       domain.RunConfig
	Plan           ComposePortPlan
	// Unverifiable maps a compose file to the reason its recorded positions can
	// no longer be trusted. Such a file contributes nothing: a stale scan would
	// declare ports the file may no longer expose.
	Unverifiable map[string]string
}

// ComposePortOutcome is the whole decision of a `run init`: the config to write,
// the rewrites to perform, and everything the recap has to account for. Nothing
// here touches disk, so the order of the steps below is testable on its own.
type ComposePortOutcome struct {
	Config  domain.RunConfig
	Merge   MergeResult
	Patches map[string][]domain.ComposePortBinding

	Written    map[string]map[string]int
	Withheld   []domain.ComposePortBinding
	Dropped    []DroppedPort
	Unreadable []domain.ComposeScan
	// Changed and Orphaned name the files that contributed nothing, and why.
	Changed  map[string]string
	Orphaned []string
}

// ResolveComposePorts runs the whole sequence in one place: build the jobs the
// selection still needs, merge, backfill the detected ports, withdraw the ones
// that cannot coexist, and keep only the rewrites whose declaration survived.
//
// The order matters and is the point of gathering it here. A mapping is only
// ever rewritten once its declaration is known to be kept — otherwise wtm would
// edit a project file for a port it then refuses to declare.
func ResolveComposePorts(params ResolveComposePortsParams) ComposePortOutcome {
	outcome := ComposePortOutcome{
		Withheld:   params.Plan.Withheld,
		Unreadable: params.Plan.Unreadable,
		Changed:    params.Unverifiable,
	}

	ports, patches := withoutFiles(params.Plan, sortedKeys(params.Unverifiable))

	forJobs := params.Answers
	forJobs.DockerComposeFiles = ComposeFilesNeedingAJob(params.Existing, params.Answers.DockerComposeFiles)
	built := BuildInitRunConfig(forJobs, params.PackageManager)
	merged, mergeResult := MergeRunConfigs(params.Existing, built)
	outcome.Merge = mergeResult

	// A file whose job was skipped — its name already taken by another file —
	// has nowhere to put its ports. Declaring them elsewhere would be a guess.
	outcome.Orphaned = orphanedComposeFiles(merged, ports)
	ports, patches = withoutFiles(ComposePortPlan{PortsByFile: ports, Patches: patches}, outcome.Orphaned)

	backfilled := BackfillDockerPorts(BackfillDockerPortsParams{Config: merged, PortsByFile: ports})
	pruned := PruneCollidingPorts(PruneCollidingPortsParams{Config: backfilled.Config, Detected: backfilled.Added})

	outcome.Config = pruned.Config
	outcome.Dropped = pruned.Dropped
	outcome.Written = RemoveDroppedPorts(backfilled.Added, pruned.Dropped)
	outcome.Patches = survivingPatches(pruned.Config, patches)
	return outcome
}

// survivingPatches keeps a rewrite only when the config that is about to be
// written still declares the variable it introduces.
func survivingPatches(cfg domain.RunConfig, patches map[string][]domain.ComposePortBinding) map[string][]domain.ComposePortBinding {
	kept := map[string][]domain.ComposePortBinding{}
	for _, file := range SortedComposeFiles(patches) {
		job := jobNamed(cfg, ComposeJobName(cfg, file))
		for _, binding := range patches[file] {
			if _, declared := job.Ports[binding.Var]; declared {
				kept[file] = append(kept[file], binding)
			}
		}
	}
	return kept
}

func orphanedComposeFiles(cfg domain.RunConfig, ports map[string]map[string]int) []string {
	var orphaned []string
	for _, file := range SortedComposeFiles(ports) {
		if len(ports[file]) > 0 && ComposeJobName(cfg, file) == "" {
			orphaned = append(orphaned, file)
		}
	}
	return orphaned
}

func withoutFiles(plan ComposePortPlan, files []string) (map[string]map[string]int, map[string][]domain.ComposePortBinding) {
	if len(files) == 0 {
		return plan.PortsByFile, plan.Patches
	}

	excluded := make(map[string]bool, len(files))
	for _, f := range files {
		excluded[f] = true
	}

	ports := map[string]map[string]int{}
	for file, p := range plan.PortsByFile {
		if !excluded[file] {
			ports[file] = p
		}
	}
	patches := map[string][]domain.ComposePortBinding{}
	for file, b := range plan.Patches {
		if !excluded[file] {
			patches[file] = b
		}
	}
	return ports, patches
}

func jobNamed(cfg domain.RunConfig, name string) domain.JobConfig {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job
		}
	}
	return domain.JobConfig{}
}
