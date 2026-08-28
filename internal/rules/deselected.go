package rules

import "github.com/LucasPcq/wtm/internal/domain"

type DeselectedJobsParams struct {
	Existing       domain.RunConfig
	PackageManager domain.PackageManager
	// DetectedScripts and DetectedComposeFiles are what the wizard displayed. A
	// job matching none of them was never offered, so nothing unchecked it.
	DetectedScripts      []domain.PackageScript
	SelectedScripts      []domain.PackageScript
	DetectedComposeFiles []string
	SelectedComposeFiles []string
	// Asked says the selection steps were displayed. Without it there is no
	// refusal to read: a non-interactive run selects only what it would have
	// pre-checked, and treating that as unchecking would delete jobs.
	Asked bool
}

// DeselectedJobs names the jobs the wizard proposed on an earlier run and the
// reader has now unchecked. It only ever reaches jobs the wizard itself owns:
// one written by `run job add` matches no detected script or file, so it is
// never a candidate for removal.
func DeselectedJobs(params DeselectedJobsParams) []string {
	if !params.Asked {
		return nil
	}

	offered := scriptJobKeys(params.PackageManager, params.DetectedScripts)
	kept := scriptJobKeys(params.PackageManager, params.SelectedScripts)

	offeredComposeJobs := composeJobsFor(params.Existing, params.DetectedComposeFiles)
	keptComposeJobs := composeJobsFor(params.Existing, params.SelectedComposeFiles)

	var deselected []string
	for _, job := range params.Existing.Jobs {
		key := scriptJobKey(job.Cmd, job.Cwd)
		if offered[key] && !kept[key] {
			deselected = append(deselected, job.Name)
			continue
		}
		// A job stacking several files (-f base.yml -f dev.yml) survives as long
		// as one of them is still checked: it is the same job, running less.
		if offeredComposeJobs[job.Name] && !keptComposeJobs[job.Name] {
			deselected = append(deselected, job.Name)
		}
	}
	return deselected
}

func scriptJobKeys(pm domain.PackageManager, scripts []domain.PackageScript) map[string]bool {
	keys := make(map[string]bool, len(scripts))
	for _, script := range scripts {
		keys[scriptJobKey(ScriptJobCmd(pm, script.Name), ScriptJobCwd(script.Workspace))] = true
	}
	return keys
}

func scriptJobKey(cmd, cwd string) string {
	return cmd + "\x00" + cwd
}

func composeJobsFor(cfg domain.RunConfig, files []string) map[string]bool {
	jobs := make(map[string]bool, len(files))
	for _, file := range files {
		if name := ComposeJobName(ComposeJobNameParams{Config: cfg, File: file}); name != "" {
			jobs[name] = true
		}
	}
	return jobs
}
