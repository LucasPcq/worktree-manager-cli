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

// The vital strip is instantaneous: it reads WorktreeStatus, not the lazily
// loaded WorktreeDetail, so it holds the panel while the rest arrives. Nothing
// here decides what to show — rules/ did — only how to stack and color it.
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

// The title row carries identity, never state: the working-tree state is a
// vital-strip chip, not a title-row pill.
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

// Wraps chip by chip and never cuts one mid-way: a truncated "origin ↑2 ↓…"
// would read as a lie rather than as a truncation.
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

// State gates WHETHER a chip is colored, Kind only WHICH color once it is:
// rules.stateChip stays the single source of truth for the strip's one
// colored chip, never re-derived here from Kind alone.
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

// Truncated before it is styled, like every other line here: trimming runes off
// an already-styled string eats its trailing reset and bleeds the color into
// whatever renders next.
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

// Carries the one thing rules.DetailSections cannot know: whether Detail has
// loaded at all yet.
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

// Which sections exist, their order and their placeholder lines are rules/'s
// job end to end; this only fits the finished stack to the panel's height.
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

// The per-section spacing here is what rules.DetailSectionChrome accounts for;
// the two must agree or the panel overflows. REVIEW's first line takes a mouse
// zone only when pr is set: the same line also renders a failure placeholder,
// and that one has nothing to open.
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

// Off the UI goroutine: PROpener shells out to gh, and calling it inside Update
// would freeze the program. Failures take openPRMsg to the output panel.
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

// A stale body renders uniformly muted: old data still on screen, visibly not
// the freshest read.
func styleText(stale bool, style lipgloss.Style, text string) string {
	if stale {
		return styles.DashboardStale.Render(text)
	}
	return style.Render(text)
}

// Empty until prsMsg lands with a real connection state, so no "unavailable"
// claim is made while the fetch is still in flight.
func (m Model) prUnavailableReason() string {
	return worktreepicker.GHBanner(m.ghConn).Title
}

// REVIEW reads the already-loaded PR list, not the lazily loaded Detail.
func (m Model) prFor(branch string) *domain.PRInfo {
	for index := range m.prs {
		if m.prs[index].Branch == branch {
			return &m.prs[index]
		}
	}
	return nil
}
