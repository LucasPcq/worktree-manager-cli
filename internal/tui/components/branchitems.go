package components

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BranchItemsParams holds inputs for BranchItems.
type BranchItemsParams struct {
	// Candidates are the branches to offer, already ordered (locals then remotes).
	Candidates []domain.BranchCandidate
	// Pinned is the value placed first and labelled with PinnedSuffix (e.g. the
	// default or base branch). Empty pins nothing. If it is not among Candidates
	// it is still added first, as a local item, so a detected default always shows.
	Pinned string
	// PinnedSuffix is appended to the pinned label (e.g. " (default)", " (base)").
	PinnedSuffix string
	// Exclude is a value omitted entirely (e.g. the worktree being reparented).
	Exclude string
}

// BranchItems builds picker rows from branch candidates with a consistent layout:
// the pinned branch first, then the remaining locals, then a separator followed by
// the remote-only branches (each tagged with a "remote" badge). Sharing this across
// the create, checkout, reparent and relocate pickers keeps remote-branch display
// identical everywhere.
func BranchItems(params BranchItemsParams) []SelectItem {
	items := make([]SelectItem, 0, len(params.Candidates)+2)

	pinnedFound := false
	for _, c := range params.Candidates {
		if c.Name == params.Pinned {
			pinnedFound = true
			items = append(items, branchItem(branchItemParams{candidate: c, suffix: params.PinnedSuffix}))
			break
		}
	}
	if params.Pinned != "" && !pinnedFound {
		items = append(items, branchItem(branchItemParams{candidate: domain.BranchCandidate{Name: params.Pinned}, suffix: params.PinnedSuffix}))
	}

	for _, c := range params.Candidates {
		if c.IsRemote || c.Name == params.Pinned || c.Name == params.Exclude {
			continue
		}
		items = append(items, branchItem(branchItemParams{candidate: c}))
	}

	hasRemote := false
	for _, c := range params.Candidates {
		if c.IsRemote && c.Name != params.Pinned && c.Name != params.Exclude {
			hasRemote = true
			break
		}
	}
	if !hasRemote {
		return items
	}

	items = append(items, SelectItem{Separator: true})
	for _, c := range params.Candidates {
		if !c.IsRemote || c.Name == params.Pinned || c.Name == params.Exclude {
			continue
		}
		items = append(items, branchItem(branchItemParams{candidate: c}))
	}
	return items
}

type branchItemParams struct {
	candidate domain.BranchCandidate
	suffix    string
}

func branchItem(params branchItemParams) SelectItem {
	c := params.candidate
	item := SelectItem{Label: c.Name + params.suffix, Value: c.Name}
	if c.IsRemote {
		item.Badges = append(item.Badges, Badge{Text: domain.BadgeTextRemote, Variant: BadgeNeutral})
	}
	if badge, ok := divergenceBadge(c); ok {
		item.Badges = append(item.Badges, badge)
	}
	return item
}

// divergenceBadge renders a local branch's ahead/behind state as a compact tag
// (e.g. "↓5", "↑2 ↓5"). Up-to-date and unknown (no origin counterpart) show none.
func divergenceBadge(c domain.BranchCandidate) (Badge, bool) {
	switch c.State {
	case domain.DivergenceBehind:
		return Badge{Text: fmt.Sprintf("%s%d", domain.BadgeGlyphBehind, c.Behind), Variant: BadgeWarning}, true
	case domain.DivergenceAhead:
		return Badge{Text: fmt.Sprintf("%s%d", domain.BadgeGlyphAhead, c.Ahead), Variant: BadgeNeutral}, true
	case domain.DivergenceDiverged:
		return Badge{Text: fmt.Sprintf("%s%d %s%d", domain.BadgeGlyphAhead, c.Ahead, domain.BadgeGlyphBehind, c.Behind), Variant: BadgeDanger}, true
	default:
		return Badge{}, false
	}
}
