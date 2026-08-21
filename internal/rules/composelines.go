package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposePatchLines describes the rewrites a patch would perform, one line
// each. Both surfaces read it: the wizard step asking for authorization and the
// recap reporting what was done — so what the user agreed to and what they are
// told happened are the same sentences.
func ComposePatchLines(byFile map[string][]domain.ComposePortBinding) []string {
	var lines []string
	for _, file := range SortedComposeFiles(byFile) {
		for _, b := range byFile[file] {
			lines = append(lines, fmt.Sprintf(domain.ComposePatchLineFmt, file, b.Service, b.Token, b.Replacement))
		}
	}
	return lines
}

// ComposeWithheldLine says why one mapping was left out of run.toml.
func ComposeWithheldLine(b domain.ComposePortBinding) string {
	if b.Status == domain.ComposePortFrozen {
		return fmt.Sprintf(domain.ComposeFrozenLineFmt, b.File, b.Service, b.Token)
	}
	return fmt.Sprintf(domain.ComposeUnsupportedFmt, b.File, b.Service, b.Reason)
}

type ComposeFixLinesParams struct {
	Binding domain.ComposePortBinding
	// Job is the run.toml job running the binding's file, when there is one. It
	// turns the second line from a placeholder into a command to paste.
	Job string
}

// ComposeFixLines is the geste the user performs to isolate a port wtm did not
// declare: the mapping to write, then the declaration that makes it effective.
// A mapping wtm cannot rewrite at all has no fix to offer.
func ComposeFixLines(params ComposeFixLinesParams) []string {
	if params.Binding.Status != domain.ComposePortFrozen || params.Binding.Replacement == "" {
		return nil
	}

	lines := []string{fmt.Sprintf(domain.ComposeFixLineFmt, params.Binding.Replacement)}
	if params.Job == "" {
		return append(lines, fmt.Sprintf(domain.ComposeFixNoJobFmt, params.Binding.Var, params.Binding.Base))
	}
	return append(lines, fmt.Sprintf(domain.ComposeFixCmdFmt, params.Job, params.Binding.Var, params.Binding.Base))
}

// ComposeDroppedLine names a detected declaration that was withdrawn and the
// one it could not coexist with.
func ComposeDroppedLine(d DroppedPort) string {
	return fmt.Sprintf(domain.ComposeDroppedLineFmt,
		d.Port.Name, d.Port.Job, d.Port.Base,
		d.Against.Name, d.Against.Job, d.Against.Base,
		d.Worktrees)
}

// ComposeUnreadableLine reports a compose file the scan could not open or parse.
func ComposeUnreadableLine(scan domain.ComposeScan) string {
	return fmt.Sprintf(domain.ComposeUnreadableFmt, scan.File, scan.Err)
}

// ComposePortsWrittenLines lists what each job gained, in the NAME=PORT form
// the rest of the run surface already speaks.
func ComposePortsWrittenLines(added map[string]map[string]int) []string {
	jobs := sortedKeys(added)
	lines := make([]string, 0, len(jobs))
	for _, job := range jobs {
		lines = append(lines, fmt.Sprintf(domain.ComposePortsWrittenFmt, job, strings.Join(PortEntries(added[job]), " ")))
	}
	return lines
}

// ComposeJobName returns the job running a compose file, matched on the same
// fragment BuildDockerJobs emits. Empty when no job runs it — a file the user
// selected but whose job they later renamed or wrote by hand.
func ComposeJobName(cfg domain.RunConfig, file string) string {
	needle := DockerComposeFileFlag(file)
	for _, job := range cfg.Jobs {
		if jobRunsComposeFile(job, needle) {
			return job.Name
		}
	}
	return ""
}

// ComposeFilesNeedingAJob narrows a selection to the files no job runs yet.
// Merging by job name alone re-adds a compose file whose job was renamed, which
// then declares the same ports twice and loses both to the collision check —
// so the file that already has a runner contributes its ports, not a second job.
func ComposeFilesNeedingAJob(cfg domain.RunConfig, files []string) []string {
	if len(cfg.Jobs) == 0 {
		return files
	}
	needing := make([]string, 0, len(files))
	for _, file := range files {
		if ComposeJobName(cfg, file) == "" {
			needing = append(needing, file)
		}
	}
	return needing
}
