package dashboard

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

func (m Model) renderDetail(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:       layout.Detail,
		Title:      domain.DashboardDetailTitle,
		TitleRight: m.detailFreshnessMarker(),
		Body:       m.detailBody(layout),
		Zone:       zoneDetail,
	})
}

// detailBody draws, in priority order: the title row (branch identity, never
// its state), the rule, the vital strip (state and the other constants —
// always instantaneous, since it reads WorktreeStatus rather than the lazily
// loaded WorktreeDetail), the blockers line when there is one, and the
// composed sections. It decides nothing about what to show — rules/ already
// did — only how to stack and color it.
func (m Model) detailBody(layout domain.DashboardLayout) []string {
	width := layout.Detail.Width - borderWidth - paddingWidth
	status, ok := m.selected()
	if !ok {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptySelection, width))}
	}
	if width <= 0 {
		return nil
	}

	stale := m.detailIsStale()
	detail, hasDetail := m.details[status.Branch]

	lines := []string{
		m.detailTitleLine(stale, status, width),
		styleText(stale, styles.DashboardRule, strings.Repeat(domain.DashboardRuleGlyph, width)),
		"",
	}
	lines = append(lines, vitalStripLines(rules.VitalChips(rules.VitalChipsParams{
		Status:       status,
		LastCommitAt: lastCommitAt(detail),
		Now:          time.Now(),
	}), width, stale)...)

	if hasDetail {
		if blocker := blockersLine(stale, detail.Blockers, width); blocker != "" {
			lines = append(lines, "", blocker)
		}
	}

	pr := m.prFor(status.Branch)
	budget := max(panelBodyHeight(layout.Detail)-len(lines), 0)
	sections := m.detailSections(detailSectionsInput{
		Status:        status,
		Detail:        detail,
		HasDetail:     hasDetail,
		Parent:        m.parents[status.Branch],
		PR:            pr,
		PRUnavailable: m.prUnavailableReason(),
		Height:        budget,
	})
	return m.appendSections(lines, sections, width, stale, pr)
}

// detailTitleLine carries identity, never state: the working-tree state is a
// vital-strip chip, not a title-row pill. DetailYouAreHere is the only thing
// that may sit to its right, and only for the worktree the cwd is under.
func (m Model) detailTitleLine(stale bool, status domain.WorktreeStatus, width int) string {
	branch := styleText(stale, styles.DashboardBranch, truncate(status.Branch, width))
	marker := m.youAreHereMarker(stale, status.Branch)
	if marker == "" {
		return branch
	}
	return spread(branch, marker, width)
}

func (m Model) youAreHereMarker(stale bool, branch string) string {
	if m.activeBranch == "" || branch != m.activeBranch {
		return ""
	}
	return styleText(stale, styles.DashboardValue, domain.DetailYouAreHere)
}

func lastCommitAt(detail domain.WorktreeDetail) time.Time {
	if len(detail.Commits) == 0 {
		return time.Time{}
	}
	return detail.Commits[0].At
}

// vitalStripLines wraps chip by chip: a chip is never cut mid-way, since a
// truncated "origin ↑2 ↓…" would read as a lie rather than a truncation. A
// chip that alone still overflows width is kept whole on its own line, for
// the same reason.
func vitalStripLines(chips []domain.Chip, width int, stale bool) []string {
	if width <= 0 || len(chips) == 0 {
		return nil
	}
	sep := styleText(stale, styles.DashboardChipSep, domain.DashboardMetaSeparator)
	sepWidth := lipgloss.Width(domain.DashboardMetaSeparator)

	var lines []string
	var current string
	currentWidth := 0
	for _, chip := range chips {
		chipWidth := lipgloss.Width(chip.Text)
		rendered := renderChip(chip, stale)

		next := currentWidth + sepWidth + chipWidth
		if current != "" && next > width {
			lines = append(lines, current)
			current, currentWidth = rendered, chipWidth
			continue
		}
		if current == "" {
			current, currentWidth = rendered, chipWidth
			continue
		}
		current, currentWidth = current+sep+rendered, next
	}
	return append(lines, current)
}

// renderChip is the vital strip's one deliberate exception to "muted is
// structure": the working-tree state chip carries the strip's only color.
// State gates WHETHER a chip is colored at all; Kind only picks WHICH color
// once State says yes — rules.stateChip is the single source of truth for
// which chip that is, this never re-derives it from Kind alone.
func renderChip(chip domain.Chip, stale bool) string {
	style := styles.DashboardChip
	if chip.State {
		switch chip.Kind {
		case domain.ChipKindDirty, domain.ChipKindRebasing:
			style = styles.Warning
		default:
			style = styles.Success
		}
	}
	return styleText(stale, style, chip.Text)
}

// blockersLine joins every refusal the worktree carries into the one line
// that answers "why can't I delete this" before the menu is even opened.
// Truncated before it is styled, like every other line in this file: trimming
// runes off an already-styled string eats its trailing reset sequence and
// bleeds the color into whatever renders after it.
func blockersLine(stale bool, blockers []domain.CleanBlocker, width int) string {
	if len(blockers) == 0 {
		return ""
	}
	labels := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		labels = append(labels, blocker.Label)
	}
	text := fmt.Sprintf(domain.DetailBlockedFmt, strings.Join(labels, domain.DashboardMetaSeparator))
	return styleText(stale, styles.DashboardBlockers, truncate(text, width))
}

// detailSectionsInput gathers what rules.DetailSections needs plus the one
// thing it cannot know: whether Detail has loaded at all yet (state 3, §8).
type detailSectionsInput struct {
	Status    domain.WorktreeStatus
	Detail    domain.WorktreeDetail
	HasDetail bool
	Parent    string
	PR        *domain.PRInfo
	// PRUnavailable names why PR data could not be read (gh missing or not
	// authenticated), empty when it is fine. Set alongside PR so a broken tool
	// never renders as "no PR" (§8 state 4).
	PRUnavailable string
	Height        int
}

// detailSections asks rules.DetailSections for the finished stack — which
// sections exist, their order, and their placeholder or failure lines when
// Detail has not loaded or a family failed to read — then only fits it to the
// panel's height. It decides nothing about what is shown: that is rules/'s
// job end to end.
func (m Model) detailSections(input detailSectionsInput) []domain.DetailSection {
	sections := rules.DetailSections(rules.DetailSectionsParams{
		Status:        input.Status,
		Detail:        input.Detail,
		DetailLoaded:  input.HasDetail,
		PR:            input.PR,
		PRUnavailable: input.PRUnavailable,
		Parent:        input.Parent,
		Height:        input.Height,
		Now:           time.Now(),
	})
	return rules.FitSections(rules.FitSectionsParams{Sections: sections, Height: input.Height})
}

// appendSections stacks each section as a blank separator, its title row
// (TitleRight flush right on that same row), a blank row, then its body
// lines — the spacing rules.DetailSectionChrome already accounts for. REVIEW's
// first line (the PR header) gets its own mouse zone when pr is set: the
// section renders that same line whether the PR is real or a failure
// placeholder, but only a real PR has anything to open.
func (m Model) appendSections(lines []string, sections []domain.DetailSection, width int, stale bool, pr *domain.PRInfo) []string {
	for _, section := range sections {
		lines = append(lines, "", sectionTitleLine(stale, section, width), "")
		for index, line := range section.Lines {
			rendered := styleText(stale, styles.DashboardValue, truncate(line, width))
			if pr != nil && section.Key == domain.DetailSectionReview && index == 0 {
				rendered = m.marks().Mark(zoneDetailPR, rendered)
			}
			lines = append(lines, rendered)
		}
	}
	return lines
}

// openPR launches the selected worktree's PR in the browser, off the UI
// goroutine: PROpener shells out to gh, and calling it inside Update would
// freeze the whole program. A failure is reported through openPRMsg, the
// same route every other operation's output takes — it never crashes the
// dashboard or blocks it.
func (m Model) openPR() (Model, tea.Cmd) {
	pr := m.prFor(m.selectedBranch())
	if pr == nil || m.params.PROpener == nil {
		return m, nil
	}
	opener, number := m.params.PROpener, pr.Number
	return m, func() tea.Msg {
		return openPRMsg{err: opener(number)}
	}
}

// sectionTitleLine mirrors panelParams.TitleRight (render.go): TitleRight is
// dropped whole, never trimmed through, when the panel is too narrow for it.
func sectionTitleLine(stale bool, section domain.DetailSection, width int) string {
	if section.TitleRight == "" {
		return styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width))
	}
	rightWidth := lipgloss.Width(section.TitleRight)
	if rightWidth+1 >= width {
		return styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width))
	}
	left := styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width-rightWidth-1))
	right := styleText(stale, styles.DashboardValue, section.TitleRight)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", max(gap, 0)) + right
}

// styleText applies style, unless the panel is stale (§8 state 2): a stale
// body renders uniformly in DashboardStale instead — old data still on
// screen, but visibly not the freshest read anymore.
func styleText(stale bool, style lipgloss.Style, text string) string {
	if stale {
		return styles.DashboardStale.Render(text)
	}
	return style.Render(text)
}

// prUnavailableReason names why PR data could not be read, reusing the same
// wording worktreepicker's own PR-aware surfaces already show for a broken
// gh — empty until prsMsg lands with an actual connection state, so no
// "unavailable" claim is made while the fetch is still in flight.
func (m Model) prUnavailableReason() string {
	return worktreepicker.GHBanner(m.ghConn).Title
}

// prFor matches the selected branch against the already-loaded PR list — no
// network call, since REVIEW does not depend on the lazily loaded Detail.
func (m Model) prFor(branch string) *domain.PRInfo {
	for index := range m.prs {
		if m.prs[index].Branch == branch {
			return &m.prs[index]
		}
	}
	return nil
}
