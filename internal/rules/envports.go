package rules

import (
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvPortBases is every port a run config declares, keyed by the job that
// carries it and its name. It is deliberately not keyed by name alone: two jobs
// may each declare a PORT, and flattening them made a link follow whichever base
// happened to be written last.
func EnvPortBases(cfg domain.RunConfig) map[domain.PortRef]int {
	bases := map[domain.PortRef]int{}
	for _, job := range cfg.Jobs {
		for name, base := range job.Ports {
			bases[domain.PortRef{Job: job.Name, Name: name}] = base
		}
	}
	return bases
}

// EnvPortBaseFor resolves the base one link follows. A link naming no job names
// no base either: the port name alone can belong to two jobs, and picking one is
// how a key came to follow the wrong port.
func EnvPortBaseFor(bases map[domain.PortRef]int, link domain.EnvPortLink) (base int, found bool) {
	if link.Job == "" {
		return 0, false
	}
	base, found = bases[domain.PortRef{Job: link.Job, Name: link.Port}]
	return base, found
}

type PlanEnvPortsParams struct {
	Links []domain.EnvPortLink
	Bases map[domain.PortRef]int
	// Offset is the worktree's port offset — zero for the main checkout, where
	// every substitution is the identity.
	Offset int
	// Lines holds the parsed .env of each file a link names, keyed by the same
	// path the link spells. A file absent from the map contributes missing keys
	// rather than nothing, so a link never disappears silently from the report.
	Lines map[string][]domain.EnvLine
}

// PlanEnvPorts resolves every link against the value its .env currently holds,
// without any I/O. A value is only ever rewritten when the declared base sits in
// it exactly once: two occurrences, or none, are reported instead — picking one
// number out of a URL by guesswork can corrupt it.
func PlanEnvPorts(params PlanEnvPortsParams) domain.EnvPortPlan {
	plan := domain.EnvPortPlan{Offset: params.Offset}

	for _, link := range params.Links {
		base, declared := EnvPortBaseFor(params.Bases, link)
		if !declared {
			continue
		}
		plan.Entries = append(plan.Entries, planEnvPortEntry(planEnvPortEntryParams{
			Link:   link,
			Base:   base,
			Offset: params.Offset,
			Lines:  params.Lines[link.File],
		}))
	}

	return plan
}

type planEnvPortEntryParams struct {
	Link   domain.EnvPortLink
	Base   int
	Offset int
	Lines  []domain.EnvLine
}

func planEnvPortEntry(params planEnvPortEntryParams) domain.EnvPortEntry {
	entry := domain.EnvPortEntry{
		File:     params.Link.File,
		Key:      params.Link.Key,
		Port:     params.Link.Port,
		Base:     params.Base,
		Resolved: params.Base + params.Offset,
	}

	line, found := envPairByKey(params.Lines, params.Link.Key)
	if !found {
		entry.Status = domain.EnvPortStatusMissingKey
		return entry
	}
	entry.CurrentValue = line.Value

	switch at := portOffsets(line.Value, entry.Base); {
	case len(at) > 1:
		entry.Status = domain.EnvPortStatusAmbiguous
	case len(at) == 1:
		entry.NewValue = replaceAt(line.Value, at[0], len(strconv.Itoa(entry.Base)), entry.Resolved)
		entry.Status = domain.EnvPortStatusRewrite
		if entry.NewValue == entry.CurrentValue {
			entry.Status = domain.EnvPortStatusUnchanged
			entry.NewValue = ""
		}
	default:
		entry.Status = alreadyResolvedStatus(line.Value, entry)
	}

	return entry
}

// alreadyResolvedStatus classifies a value the base does not appear in. Finding
// the resolved port instead means a previous run already applied the offset —
// the common case when `wtm env` runs twice — and is not an anomaly.
func alreadyResolvedStatus(value string, entry domain.EnvPortEntry) domain.EnvPortStatus {
	if entry.Resolved == entry.Base {
		return domain.EnvPortStatusNotFound
	}
	switch len(portOffsets(value, entry.Resolved)) {
	case 0:
		return domain.EnvPortStatusNotFound
	case 1:
		return domain.EnvPortStatusUnchanged
	default:
		return domain.EnvPortStatusAmbiguous
	}
}

// ApplyEnvPorts writes the planned rewrites into the lines of one file. Raw is
// cleared on every mutated pair, which is what makes RenderEnv re-emit the line
// from Key/Value instead of reproducing the stale original.
func ApplyEnvPorts(lines []domain.EnvLine, entries []domain.EnvPortEntry) []domain.EnvLine {
	byKey := map[string]string{}
	for _, e := range entries {
		if e.Status == domain.EnvPortStatusRewrite {
			byKey[e.Key] = e.NewValue
		}
	}
	if len(byKey) == 0 {
		return lines
	}

	out := make([]domain.EnvLine, len(lines))
	copy(out, lines)
	for i, line := range out {
		value, rewritten := byKey[line.Key]
		if line.Kind != domain.EnvLinePair || !rewritten {
			continue
		}
		out[i].Value = value
		out[i].Raw = ""
	}
	return out
}

// EnvPortBasesByKey is the base each linked .env key follows, keyed by the key
// itself — what the reconciliation diff needs to compare two worktrees' values.
func EnvPortBasesByKey(links []domain.EnvPortLink, bases map[domain.PortRef]int) map[string]int {
	byKey := map[string]int{}
	for _, link := range links {
		if base, declared := EnvPortBaseFor(bases, link); declared {
			byKey[link.Key] = base
		}
	}
	return byKey
}

type ReduceEnvPortParams struct {
	Value string
	Base  int
	Block int
}

// ReduceEnvPortValue rewinds whichever worktree's port a value holds back to the
// declared base, so two worktrees' spellings of the same setting compare equal.
//
// It cannot simply undo the reader's own offset: under the `parent` strategy the
// source value comes from another worktree, bound on that worktree's offset,
// which the reader does not know. What every worktree's port does share is the
// arithmetic — it is the base plus some multiple of the block — so the reduction
// looks for a number of that shape instead of for one known value.
//
// A value holding no such number, or more than one, is returned untouched:
// reducing on a guess would hide a real conflict.
func ReduceEnvPortValue(params ReduceEnvPortParams) string {
	block := params.Block
	if block <= 0 {
		block = domain.PortOffsetBlock
	}

	var at []int
	for _, n := range standaloneNumbers(params.Value) {
		gap := n.value - params.Base
		if gap < 0 || gap%block != 0 || gap/block > domain.PortCollisionHorizon {
			continue
		}
		at = append(at, n.at)
	}
	if len(at) != 1 {
		return params.Value
	}

	number := params.Value[at[0]:]
	return replaceAt(params.Value, at[0], numberLengthAt(number), params.Base)
}

type standaloneNumber struct {
	at    int
	value int
}

// standaloneNumbers finds every run of digits in value that is not part of a
// longer one, which is the same boundary rule the substitution uses.
func standaloneNumbers(value string) []standaloneNumber {
	var out []standaloneNumber
	for i := 0; i < len(value); {
		if !isDigitAt(value, i) {
			i++
			continue
		}
		end := i
		for isDigitAt(value, end) {
			end++
		}
		if n, err := strconv.Atoi(value[i:end]); err == nil {
			out = append(out, standaloneNumber{at: i, value: n})
		}
		i = end
	}
	return out
}

func numberLengthAt(value string) int {
	end := 0
	for isDigitAt(value, end) {
		end++
	}
	return end
}

// portOffsets locates port in value as a standalone number. The digit boundaries
// are the whole point: without them base 5432 matches inside 54321 and the
// substitution silently corrupts the value.
func portOffsets(value string, port int) []int {
	needle := strconv.Itoa(port)

	var offsets []int
	for from := 0; ; {
		i := strings.Index(value[from:], needle)
		if i < 0 {
			return offsets
		}
		at := from + i
		if !isDigitAt(value, at-1) && !isDigitAt(value, at+len(needle)) {
			offsets = append(offsets, at)
		}
		from = at + len(needle)
	}
}

func isDigitAt(value string, i int) bool {
	return i >= 0 && i < len(value) && value[i] >= '0' && value[i] <= '9'
}

// replaceAt swaps the width-character number at offset at for to.
func replaceAt(value string, at int, width int, to int) string {
	return value[:at] + strconv.Itoa(to) + value[at+width:]
}

func envPairByKey(lines []domain.EnvLine, key string) (domain.EnvLine, bool) {
	for _, line := range lines {
		if line.Kind == domain.EnvLinePair && line.Key == key {
			return line, true
		}
	}
	return domain.EnvLine{}, false
}

// ElideEnvValue shortens a value for display. Cutting at the credentials
// separator is not only about width: a DATABASE_URL printed whole puts a
// password on screen, and the part worth reading is the host and the port.
func ElideEnvValue(value string) string {
	if at := strings.LastIndex(value, domain.EnvCredentialsSeparator); at >= 0 {
		value = domain.Ellipsis + value[at:]
	}
	if len(value) <= domain.EnvValueDisplayWidth {
		return value
	}
	return value[:domain.EnvValueDisplayWidth-len(domain.Ellipsis)] + domain.Ellipsis
}

type EnvPortCandidatesParams struct {
	// Lines is each configured env target's parsed content, keyed by target.
	Lines map[string][]domain.EnvLine
	Bases map[domain.PortRef]int
	// Existing are the links run.toml already declares, which are never offered
	// again — a re-run of the detection is additive, like the compose one.
	Existing []domain.EnvPortLink
}

// EnvPortCandidates finds the .env keys whose value holds exactly one declared
// base port. A value matching two different ports is not offered: one link moves
// one number, so wtm would have to pick which service the key belongs to.
func EnvPortCandidates(params EnvPortCandidatesParams) []domain.EnvPortLink {
	declared := map[domain.EnvPortLink]bool{}
	for _, link := range params.Existing {
		declared[domain.EnvPortLink{File: link.File, Key: link.Key}] = true
	}

	var candidates []domain.EnvPortLink
	for _, file := range sortedKeys(params.Lines) {
		for _, line := range params.Lines[file] {
			if line.Kind != domain.EnvLinePair || declared[domain.EnvPortLink{File: file, Key: line.Key}] {
				continue
			}
			if ref, sole := solePortIn(line.Value, params.Bases); sole {
				candidates = append(candidates, domain.EnvPortLink{
					File: file, Key: line.Key, Job: ref.Job, Port: ref.Name,
				})
			}
		}
	}
	return candidates
}

// solePortIn names the one declared port whose base sits exactly once in value.
// Two ports sharing a base cannot both be it, so neither is offered.
func solePortIn(value string, bases map[domain.PortRef]int) (ref domain.PortRef, sole bool) {
	for _, candidate := range sortedPortRefs(bases) {
		if len(portOffsets(value, bases[candidate])) != 1 {
			continue
		}
		if sole {
			return domain.PortRef{}, false
		}
		ref, sole = candidate, true
	}
	return ref, sole
}

// sortedPortRefs orders the declarations so a match is picked the same way twice.
func sortedPortRefs(bases map[domain.PortRef]int) []domain.PortRef {
	refs := make([]domain.PortRef, 0, len(bases))
	for ref := range bases {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Job != refs[j].Job {
			return refs[i].Job < refs[j].Job
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

// EnvPortPlanTouches reports whether a plan rewrites anything in one env target.
func EnvPortPlanTouches(plan domain.EnvPortPlan, target string) bool {
	for _, e := range plan.Rewrites() {
		if e.File == target {
			return true
		}
	}
	return false
}
