package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type VitalChipsParams struct {
	Status       domain.WorktreeStatus
	LastCommitAt time.Time
	Now          time.Time
}

// VitalChips builds the vital strip. State comes first — it is the fastest
// read — and it is the only coloured chip. `created` never appears here: it
// belongs to LINKS.
func VitalChips(params VitalChipsParams) []domain.Chip {
	chips := []domain.Chip{stateChip(params.Status)}

	if params.Status.CommitsAhead > 0 {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipBaseFmt, params.Status.CommitsAhead),
			Kind: domain.ChipKindNeutral,
		})
	}
	if origin := originChipText(params.Status); origin != "" {
		chips = append(chips, domain.Chip{Text: origin, Kind: domain.ChipKindNeutral})
	}
	if age := RelativeAge(RelativeAgeParams{At: params.LastCommitAt, Now: params.Now}); age != "" {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipActiveFmt, age),
			Kind: domain.ChipKindNeutral,
		})
	}
	return chips
}

// stateChip: a paused rebase outranks "dirty". A mid-rebase branch is always
// dirty too, but the rebase is what is actionable.
func stateChip(status domain.WorktreeStatus) domain.Chip {
	if status.RebaseInProgress {
		return domain.Chip{Text: domain.ChipRebasing, State: true, Kind: domain.ChipKindRebasing}
	}
	if status.IsDirty {
		return domain.Chip{Text: domain.ChipDirty, State: true, Kind: domain.ChipKindDirty}
	}
	return domain.Chip{Text: domain.ChipClean, State: true, Kind: domain.ChipKindClean}
}

func originChipText(status domain.WorktreeStatus) string {
	switch status.OriginState {
	case domain.DivergenceUnknown:
		// No origin counterpart to compare against: nothing to show.
		return ""
	case domain.DivergenceUpToDate:
		// Same commit as origin is the non-event the strip must not recite.
		return ""
	case domain.DivergenceDiverged:
		return fmt.Sprintf(domain.ChipOriginDivergedFmt, status.OriginAhead, status.OriginBehind)
	case domain.DivergenceAhead:
		return fmt.Sprintf(domain.ChipOriginAheadFmt, status.OriginAhead)
	case domain.DivergenceBehind:
		return fmt.Sprintf(domain.ChipOriginBehindFmt, status.OriginBehind)
	default:
		return ""
	}
}

type DetailSectionsParams struct {
	Status domain.WorktreeStatus
	Detail domain.WorktreeDetail
	PR     *domain.PRInfo
	Parent string
	Height int
	// Now anchors every relative age this call renders (ACTIVITY's "… ago"), so
	// the whole panel is reproducible from its inputs instead of reading the
	// wall clock mid-build.
	Now time.Time
}

// DetailSections emits the sections in a fixed order, and only the ones that
// have something to say: a section's position varies between worktrees, its
// rank never does.
func DetailSections(params DetailSectionsParams) []domain.DetailSection {
	sections := make([]domain.DetailSection, 0, 4)

	if params.PR != nil {
		sections = append(sections, reviewSection(*params.PR))
	}
	if changed := changedCount(params.Detail.Changes); changed > 0 {
		sections = append(sections, changesSection(params.Detail.Changes, listBudget(params, true)))
	}
	if len(params.Detail.Commits) > 0 {
		sections = append(sections, activitySection(params.Detail.Commits, listBudget(params, false), params.Now))
	}
	return append(sections, linksSection(params))
}

func changedCount(changes domain.WorkingChanges) int {
	return changes.Modified + changes.Untracked + changes.Staged
}

// listBudget splits the remaining height between CHANGES and ACTIVITY: a dirty
// worktree gives it to the files (what you're working on), a clean one to the
// commits (what the branch is).
func listBudget(params DetailSectionsParams, forChanges bool) int {
	room := max(params.Height-domain.DetailFixedRows, domain.DetailMinListRows)
	dirty := changedCount(params.Detail.Changes) > 0 && params.Status.IsDirty
	if forChanges == dirty {
		return room - domain.DetailMinListRows
	}
	return domain.DetailMinListRows
}

// splitBudget divides a list of `total` items into what's shown and what's
// folded into a "… N more" line, reserving one row for that line once the
// budget is exceeded.
func splitBudget(total, budget int) (shown, more int) {
	if budget <= 0 {
		return 0, total
	}
	if total <= budget {
		return total, 0
	}
	return budget - 1, total - (budget - 1)
}

func reviewSection(pr domain.PRInfo) domain.DetailSection {
	return domain.DetailSection{
		Key:   domain.DetailSectionReview,
		Title: domain.DetailSectionReview,
		Lines: []string{fmt.Sprintf(domain.DetailReviewHeaderFmt, pr.Number, pr.Title, pr.State)},
	}
}

func changesSection(changes domain.WorkingChanges, budget int) domain.DetailSection {
	var lines []string

	shown, more := splitBudget(len(changes.Files), budget)
	for _, file := range changes.Files[:shown] {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailFileFmt, fileGlyph(file), file.Path))
	}
	if more > 0 {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailMoreFmt, more))
	}

	return domain.DetailSection{
		Key:        domain.DetailSectionChanges,
		Title:      domain.DetailSectionChanges,
		TitleRight: changesSummary(changes),
		Lines:      lines,
	}
}

func changesSummary(changes domain.WorkingChanges) string {
	var parts []string
	if changes.Modified > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesModifiedFmt, changes.Modified))
	}
	if changes.Untracked > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesUntrackedFmt, changes.Untracked))
	}
	if changes.Staged > 0 {
		parts = append(parts, fmt.Sprintf(domain.ChangesStagedFmt, changes.Staged))
	}
	summary := strings.Join(parts, domain.DashboardMetaSeparator)

	if changes.Insertions == 0 && changes.Deletions == 0 {
		return summary
	}
	diff := fmt.Sprintf(domain.ChangesDiffStatFmt, changes.Insertions, changes.Deletions)
	if summary == "" {
		return diff
	}
	return summary + domain.DetailListIndent + diff
}

// fileGlyph reads the porcelain XY code as-is: the worktree column (Y) wins
// when set, since it's what you'd act on next; the index column (X) is the
// fallback for a purely staged change.
func fileGlyph(file domain.PorcelainEntry) string {
	if file.Status == domain.PorcelainUntracked {
		return domain.DetailUntrackedGlyph
	}
	if len(file.Status) < 2 {
		return domain.DetailUntrackedGlyph
	}
	if rune(file.Status[1]) != domain.PorcelainUnmodified {
		return string(file.Status[1])
	}
	return string(file.Status[0])
}

func activitySection(commits []domain.CommitSummary, budget int, now time.Time) domain.DetailSection {
	shown, more := splitBudget(len(commits), budget)

	lines := make([]string, 0, shown+1)
	for _, commit := range commits[:shown] {
		lines = append(lines, domain.DetailListIndent+commitLine(commit, now))
	}
	if more > 0 {
		lines = append(lines, domain.DetailListIndent+fmt.Sprintf(domain.DetailMoreFmt, more))
	}

	return domain.DetailSection{Key: domain.DetailSectionActivity, Title: domain.DetailSectionActivity, Lines: lines}
}

// commitLine reuses commit.SHA as-is: infra.RecentCommits already asks git for
// its abbreviated `%h`, so re-truncating it here would only risk cutting an
// abbreviation git had to grow to stay unambiguous.
func commitLine(commit domain.CommitSummary, now time.Time) string {
	line := fmt.Sprintf(domain.DetailCommitFmt, commit.SHA, commit.Subject)
	if meta := commitMeta(commit, now); meta != "" {
		line += domain.DetailListIndent + meta
	}
	return line
}

// commitMeta never formats a zero CommitSummary.At as a date: RelativeAge
// already returns "" for it, meaning unknown, never the epoch.
func commitMeta(commit domain.CommitSummary, now time.Time) string {
	var parts []string
	if commit.Author != "" {
		parts = append(parts, commit.Author)
	}
	if age := RelativeAge(RelativeAgeParams{At: commit.At, Now: now}); age != "" {
		parts = append(parts, age)
	}
	return strings.Join(parts, domain.DashboardMetaSeparator)
}

func linksSection(params DetailSectionsParams) domain.DetailSection {
	var lines []string

	if params.Parent != "" {
		lines = append(lines, fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelParent, params.Parent))
	}
	if len(params.Detail.Children) > 0 {
		lines = append(lines, fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelChildren,
			strings.Join(params.Detail.Children, domain.DetailListSep)))
	}
	if age := RelativeAge(RelativeAgeParams{At: params.Status.CreatedAt, Now: params.Now}); age != "" {
		lines = append(lines, fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelCreated, age))
	}
	if env := envLine(params.Detail); env != "" {
		lines = append(lines, fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelEnv, env))
	}
	lines = append(lines, fmt.Sprintf(domain.DetailFieldFmt, domain.DashboardLabelPath, params.Status.Path))

	return domain.DetailSection{Key: domain.DetailSectionLinks, Title: domain.DetailSectionLinks, Lines: lines}
}

// envLine distinguishes three situations the LINKS "Env" field can be in: a
// family that failed to read says why; a legitimate absence (no env files
// declared) says so without an alert glyph; a configured family with no drift
// has nothing to say and is omitted, like an up-to-date origin.
func envLine(detail domain.WorktreeDetail) string {
	if err, failed := detail.Failures[domain.DetailFamilyEnv]; failed {
		return fmt.Sprintf(domain.DashboardUnavailableFmt, err)
	}
	if !detail.EnvDrift.Configured {
		return domain.DashboardNotConfigured
	}
	return envDriftSummary(detail.EnvDrift)
}

func envDriftSummary(drift domain.EnvDriftSummary) string {
	var parts []string
	if drift.Missing > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvMissingFmt, drift.Missing))
	}
	if drift.Conflicting > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvConflictingFmt, drift.Conflicting))
	}
	if drift.Orphan > 0 {
		parts = append(parts, fmt.Sprintf(domain.EnvOrphanFmt, drift.Orphan))
	}
	return strings.Join(parts, domain.DashboardMetaSeparator)
}

type FitSectionsParams struct {
	Sections []domain.DetailSection
	Height   int
}

// FitSections drops sections from the end of DetailSectionDropOrder while the
// stack exceeds the height. The vital strip and the blockers line never come
// through here: they don't fall.
func FitSections(params FitSectionsParams) []domain.DetailSection {
	kept := params.Sections
	for sectionsHeight(kept) > params.Height && len(kept) > 0 {
		kept = dropLowestPriority(kept)
	}
	return kept
}

// sectionsHeight counts, per section, its title, the blank line under it and
// its body lines.
func sectionsHeight(sections []domain.DetailSection) int {
	total := 0
	for _, section := range sections {
		total += domain.DetailSectionChrome + len(section.Lines)
	}
	return total
}

func dropLowestPriority(sections []domain.DetailSection) []domain.DetailSection {
	for i := len(domain.DetailSectionDropOrder) - 1; i >= 0; i-- {
		key := domain.DetailSectionDropOrder[i]
		for index, section := range sections {
			if section.Key == key {
				return append(sections[:index:index], sections[index+1:]...)
			}
		}
	}
	return nil
}
