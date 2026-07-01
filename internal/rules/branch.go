package rules

import (
	"slices"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ClassifyDivergence maps a local branch's ahead/behind commit counts (relative
// to its origin counterpart) to a DivergenceState.
func ClassifyDivergence(ahead, behind int) domain.DivergenceState {
	if ahead > 0 && behind > 0 {
		return domain.DivergenceDiverged
	}
	if behind > 0 {
		return domain.DivergenceBehind
	}
	if ahead > 0 {
		return domain.DivergenceAhead
	}
	return domain.DivergenceUpToDate
}

// IsRemoteBranch reports whether name is an origin remote-tracking ref
// ("origin/feature") rather than a bare local branch name.
func IsRemoteBranch(name string) bool {
	return strings.HasPrefix(name, domain.RemoteBranchPrefix)
}

// BranchCandidateExists reports whether ref matches a known start-point (a local
// branch or a remote-tracking ref like origin/x) in candidates.
func BranchCandidateExists(candidates []domain.BranchCandidate, ref string) bool {
	return slices.ContainsFunc(candidates, func(c domain.BranchCandidate) bool {
		return c.Name == ref
	})
}

// ShouldOfferFastForward reports whether a source branch in the given divergence
// state can be cleanly fast-forwarded to origin — true only when it is strictly
// behind (diverged branches are not fast-forwardable and keep their local commits).
func ShouldOfferFastForward(state domain.DivergenceState) bool {
	return state == domain.DivergenceBehind
}

// MergeBranchCandidatesParams holds inputs for MergeBranchCandidates. Local and
// Remote are git ref short names ("feature", "origin/feature"). Divergence maps
// a local branch name to its ahead/behind counts versus origin; locals absent
// from the map keep the zero/Unknown state.
type MergeBranchCandidatesParams struct {
	Local      []string
	Remote     []string
	Divergence map[string]domain.AheadBehind
}

// MergeBranchCandidates combines local and remote-tracking branches into a
// single ordered candidate set for branch pickers. Locals come first (sorted),
// then remote-only branches (sorted): a remote whose stripped name already
// exists locally is dropped so the local branch wins. Each local branch with a
// Divergence entry is tagged with its ahead/behind counts and classified state.
func MergeBranchCandidates(params MergeBranchCandidatesParams) []domain.BranchCandidate {
	localSet := make(map[string]struct{}, len(params.Local))
	for _, b := range params.Local {
		localSet[b] = struct{}{}
	}

	locals := append([]string(nil), params.Local...)
	sort.Strings(locals)

	remoteOnly := make([]string, 0, len(params.Remote))
	for _, r := range params.Remote {
		name := strings.TrimPrefix(r, domain.RemoteBranchPrefix)
		if _, ok := localSet[name]; ok {
			continue
		}
		remoteOnly = append(remoteOnly, r)
	}
	sort.Strings(remoteOnly)

	candidates := make([]domain.BranchCandidate, 0, len(locals)+len(remoteOnly))
	for _, b := range locals {
		candidate := domain.BranchCandidate{Name: b, IsRemote: false}
		if d, ok := params.Divergence[b]; ok {
			candidate.Ahead = d.Ahead
			candidate.Behind = d.Behind
			candidate.State = ClassifyDivergence(d.Ahead, d.Behind)
		}
		candidates = append(candidates, candidate)
	}
	for _, r := range remoteOnly {
		candidates = append(candidates, domain.BranchCandidate{Name: r, IsRemote: true})
	}
	return candidates
}
