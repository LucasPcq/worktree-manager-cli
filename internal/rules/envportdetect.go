package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsPortKey reports whether an env key names a port by the only convention wtm
// reads: PORT, or a name ending in _PORT. It is deliberately not a guess about
// what a framework does — it is the .env equivalent of a compose `ports:` list.
func IsPortKey(key string) bool {
	if !IsEnvVarName(key) {
		return false
	}
	return key == domain.EnvPortKeyName || strings.HasSuffix(key, domain.EnvPortKeySuffix)
}

// ExtractEnvPorts keeps the pairs that name a port and carry a usable base. A
// placeholder or a URL never survives: the value has to be a bare number in
// range, which is what makes reading an uncommitted .env.example safe.
func ExtractEnvPorts(lines []domain.EnvLine) map[string]int {
	ports := map[string]int{}
	for _, line := range lines {
		if line.Kind != domain.EnvLinePair || !IsPortKey(line.Key) {
			continue
		}
		base, err := strconv.Atoi(strings.TrimSpace(line.Value))
		if err != nil || base < domain.PortMin || base > domain.PortMax {
			continue
		}
		ports[line.Key] = base
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

// EnvPortSource is one file's contribution to a directory, in increasing order
// of priority: the caller lists the weakest first.
type EnvPortSource struct {
	File  string
	Ports map[string]int
}

type MergeEnvPortSourcesParams struct {
	Dir     string
	Sources []EnvPortSource
}

// MergeEnvPortSources collapses a directory's env files into one scan, later
// sources winning, and records which file each surviving port came from.
func MergeEnvPortSources(params MergeEnvPortSourcesParams) domain.EnvPortScan {
	scan := domain.EnvPortScan{Dir: params.Dir}

	for _, source := range params.Sources {
		for _, name := range sortedPortNames(source.Ports) {
			if scan.Ports == nil {
				scan.Ports = map[string]int{}
				scan.SourceByVar = map[string]string{}
			}
			scan.Ports[name] = source.Ports[name]
			scan.SourceByVar[name] = source.File
		}
	}

	return scan
}

type BackfillScriptPortsParams struct {
	Config         domain.RunConfig
	PackageManager domain.PackageManager
	// Scripts are the ones the user selected. Matching on them rather than on
	// the job's cwd alone is what keeps the root .env off a docker-compose job,
	// which shares that cwd.
	Scripts    []domain.PackageScript
	ScansByDir map[string]domain.EnvPortScan
}

type BackfillScriptPortsResult struct {
	Config domain.RunConfig
	Added  map[string]map[string]int
	// Sources mirrors Added with the file each port was read from.
	Sources map[string]map[string]string
}

// BackfillScriptPorts gives each dev server job the ports declared in the env
// files of the directory it runs in, matched on the same "<pm> run <script>"
// command and cwd BuildScriptJobs emits. Additive by construction: a variable
// the job already declares keeps its value, so re-running `run init` is safe.
func BackfillScriptPorts(params BackfillScriptPortsParams) BackfillScriptPortsResult {
	result := BackfillScriptPortsResult{
		Config:  params.Config,
		Added:   map[string]map[string]int{},
		Sources: map[string]map[string]string{},
	}
	result.Config.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(result.Config.Jobs, params.Config.Jobs)

	for _, script := range params.Scripts {
		if script.Kind != domain.JobKindService {
			continue
		}

		cwd := ScriptJobCwd(script.Workspace)
		scan, found := params.ScansByDir[cwd]
		if !found || len(scan.Ports) == 0 {
			continue
		}

		cmd := ScriptJobCmd(params.PackageManager, script.Name)
		for i, job := range result.Config.Jobs {
			if job.Cmd != cmd || job.Cwd != cwd {
				continue
			}
			for _, name := range sortedPortNames(scan.Ports) {
				if _, declared := result.Config.Jobs[i].Ports[name]; declared {
					continue
				}
				// Cloned on first write: the slice copy above shares every job's
				// Ports map with the config the caller passed in.
				result.Config.Jobs[i].Ports = clonePorts(result.Config.Jobs[i].Ports)
				result.Config.Jobs[i].Ports[name] = scan.Ports[name]

				if result.Added[job.Name] == nil {
					result.Added[job.Name] = map[string]int{}
					result.Sources[job.Name] = map[string]string{}
				}
				result.Added[job.Name][name] = scan.Ports[name]
				result.Sources[job.Name][name] = scan.SourceByVar[name]
			}
		}
	}

	return result
}

type EnvPortsWrittenLinesParams struct {
	Written map[string]map[string]int
	Sources map[string]map[string]string
}

// EnvPortsWrittenLines renders the recap body: one line per declared port,
// naming the file it was read from so the user can go check the value.
func EnvPortsWrittenLines(params EnvPortsWrittenLinesParams) []string {
	var lines []string
	for _, job := range sortedKeys(params.Written) {
		for _, name := range sortedPortNames(params.Written[job]) {
			lines = append(lines, fmt.Sprintf(
				domain.EnvPortDetectedLineFmt, job, name, params.Written[job][name], params.Sources[job][name],
			))
		}
	}
	return lines
}

// PortsIgnoredLines names the jobs whose command never mentions the port wtm
// injects. It proves nothing — a command can read the variable on its own — so
// it names the job and the variable and leaves the conclusion to the reader.
func PortsIgnoredLines(fixes []domain.JobCmdFix) []string {
	lines := make([]string, 0, len(fixes)+2)
	for _, fix := range fixes {
		lines = append(lines, fmt.Sprintf(domain.PortsIgnoredLineFmt, fix.Job, strings.Join(fix.Vars, ", "), fix.Cmd))
	}
	return append(lines, "", domain.PortsIgnoredHint)
}
