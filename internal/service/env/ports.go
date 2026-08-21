package env

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvPortsParams locates the .env files a set of links names and the offset the
// worktree binds on. Bases comes from the run config, so this package never
// reads run.toml itself.
type EnvPortsParams struct {
	WorktreePath string
	Links        []domain.EnvPortLink
	Bases        map[string]int
	Offset       int
	// Block is the spacing between two worktrees' ports, which the reconciliation
	// diff needs to recognize another worktree's spelling of the same port.
	Block int
}

// Empty reports whether the project declares no link at all, which is the common
// case and means every caller can skip the whole reconciliation.
func (p EnvPortsParams) Empty() bool { return len(p.Links) == 0 }

// ComputeEnvPorts resolves every link against the worktree's current .env files,
// writing nothing. A file that does not exist yields missing keys rather than
// silence — a link the user declared must stay visible in the report.
func ComputeEnvPorts(params EnvPortsParams) (domain.EnvPortPlan, error) {
	lines, err := readLinkedFiles(params)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}

	return rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:  params.Links,
		Bases:  params.Bases,
		Offset: params.Offset,
		Lines:  lines,
	}), nil
}

// ApplyEnvPorts recomputes the plan and writes the rewrites it holds, one file at
// a time. It returns the plan it acted on, so a caller reports what happened
// rather than what it intended.
func ApplyEnvPorts(params EnvPortsParams) (domain.EnvPortPlan, error) {
	plan, err := ComputeEnvPorts(params)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}

	for _, file := range rewrittenFiles(plan) {
		path := filepath.Join(params.WorktreePath, file)
		lines, readErr := readEnvFile(path)
		if readErr != nil {
			return domain.EnvPortPlan{}, readErr
		}

		rendered := rules.RenderEnv(rules.ApplyEnvPorts(lines, entriesForFile(plan, file)))
		if rendered == rules.RenderEnv(lines) {
			continue
		}
		if writeErr := writeEnvFile(path, rendered); writeErr != nil {
			return domain.EnvPortPlan{}, writeErr
		}
	}

	return plan, nil
}

// EnvPortBasesFor is the base each linked key of one env target follows, which
// the reconciliation diff needs to compare two worktrees' values.
func EnvPortBasesFor(params EnvPortsParams, target string) map[string]int {
	return rules.EnvPortBasesByKey(linksForFile(params.Links, target), params.Bases)
}

func readLinkedFiles(params EnvPortsParams) (map[string][]domain.EnvLine, error) {
	lines := map[string][]domain.EnvLine{}
	for _, link := range params.Links {
		if _, read := lines[link.File]; read {
			continue
		}
		parsed, err := readEnvFile(filepath.Join(params.WorktreePath, link.File))
		if err != nil {
			return nil, err
		}
		lines[link.File] = parsed
	}
	return lines, nil
}

func rewrittenFiles(plan domain.EnvPortPlan) []string {
	var files []string
	seen := map[string]bool{}
	for _, entry := range plan.Rewrites() {
		if seen[entry.File] {
			continue
		}
		seen[entry.File] = true
		files = append(files, entry.File)
	}
	return files
}

func entriesForFile(plan domain.EnvPortPlan, file string) []domain.EnvPortEntry {
	var out []domain.EnvPortEntry
	for _, entry := range plan.Entries {
		if entry.File == file {
			out = append(out, entry)
		}
	}
	return out
}

func linksForFile(links []domain.EnvPortLink, file string) []domain.EnvPortLink {
	var out []domain.EnvPortLink
	for _, link := range links {
		if link.File == file {
			out = append(out, link)
		}
	}
	return out
}
