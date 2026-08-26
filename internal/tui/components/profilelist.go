package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	profileNameCharLimit = 40
	profileNameWidth     = 32
	// noMark is the cursor position no row is marked at.
	noMark = -1
)

// ProfileListModel edits the split `run up` will offer. The proposal it starts
// from is a guess about an intention — grouping packages into apps is something
// only the user knows — so every operation here exists to correct it.
type ProfileListModel struct {
	profiles []domain.ProfileConfig
	cursor   int
	mark     int
	width    int
	height   int
	title    string
	desc     string
	done     bool
	aborted  bool

	naming    bool
	namingNew bool
	input     textinput.Model
	err       string
}

type NewProfileListParams struct {
	Title       string
	Description string
	Profiles    []domain.ProfileConfig
}

func NewProfileList(params NewProfileListParams) ProfileListModel {
	// Copied, never aliased: the proposal it starts from can be the loaded
	// config's own slice, and renaming a profile through it edited run.toml —
	// a rename the user then escaped out of survived.
	profiles := make([]domain.ProfileConfig, len(params.Profiles))
	copy(profiles, params.Profiles)

	return ProfileListModel{
		profiles: profiles,
		mark:     noMark,
		title:    params.Title,
		desc:     params.Description,
		width:    80,
	}
}

func (m ProfileListModel) Profiles() []domain.ProfileConfig { return m.profiles }
func (m ProfileListModel) Done() bool                       { return m.done }
func (m ProfileListModel) Aborted() bool                    { return m.aborted }
func (m ProfileListModel) Init() tea.Cmd                    { return nil }

func (m *ProfileListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m ProfileListModel) doneRow() int { return len(m.profiles) }

func (m ProfileListModel) Update(msg tea.Msg) (ProfileListModel, tea.Cmd) {
	if m.naming {
		return m.updateNaming(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < m.doneRow() {
			m.cursor++
		}
	case domain.ProfileListKeyRemove:
		return m.remove(), nil
	case domain.ProfileListKeyMerge:
		return m.mergeStep(), nil
	case domain.ProfileListKeyRename:
		if m.cursor < m.doneRow() {
			return m.startNaming(false), nil
		}
	case domain.ProfileListKeyNew:
		return m.startNaming(true), nil
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
		}
	case "esc":
		if m.mark != noMark {
			m.mark = noMark
			return m, nil
		}
		m.aborted = true
	}

	return m, nil
}

func (m ProfileListModel) remove() ProfileListModel {
	if m.cursor >= m.doneRow() {
		return m
	}
	m.profiles = append(append([]domain.ProfileConfig{}, m.profiles[:m.cursor]...), m.profiles[m.cursor+1:]...)
	m.mark = noMark
	if m.cursor > 0 && m.cursor >= len(m.profiles) {
		m.cursor = len(m.profiles)
	}
	return m.ensureDefault()
}

// mergeStep is both halves of a merge: the first press marks the row the jobs
// will land in, the second folds the row under the cursor into it.
func (m ProfileListModel) mergeStep() ProfileListModel {
	if m.cursor >= m.doneRow() {
		return m
	}
	if m.mark == noMark {
		m.mark = m.cursor
		return m
	}
	if m.mark == m.cursor {
		m.mark = noMark
		return m
	}

	target, source := m.mark, m.cursor
	merged := m.profiles[target]
	merged.Jobs = unionJobs(merged.Jobs, m.profiles[source].Jobs)
	merged.Default = merged.Default || m.profiles[source].Default

	kept := make([]domain.ProfileConfig, 0, len(m.profiles)-1)
	for i, profile := range m.profiles {
		switch i {
		case target:
			kept = append(kept, merged)
		case source:
		default:
			kept = append(kept, profile)
		}
	}

	m.profiles = kept
	m.mark = noMark
	if m.cursor >= len(m.profiles) {
		m.cursor = len(m.profiles)
	}
	return m.ensureDefault()
}

// unionJobs keeps the target's order and appends what the source adds. A job
// both profiles carry — the shared infrastructure — must not be started twice.
func unionJobs(target, source []string) []string {
	seen := make(map[string]bool, len(target))
	for _, job := range target {
		seen[job] = true
	}
	merged := append([]string{}, target...)
	for _, job := range source {
		if !seen[job] {
			seen[job] = true
			merged = append(merged, job)
		}
	}
	return merged
}

// ensureDefault keeps exactly one profile marked: the picker pre-selects it,
// and losing it would leave the picker without a starting point.
func (m ProfileListModel) ensureDefault() ProfileListModel {
	if len(m.profiles) == 0 {
		return m
	}
	for _, profile := range m.profiles {
		if profile.Default {
			return m
		}
	}
	m.profiles[0].Default = true
	return m
}

func (m ProfileListModel) startNaming(creating bool) ProfileListModel {
	input := textinput.New()
	input.CharLimit = profileNameCharLimit
	input.Width = profileNameWidth
	if !creating {
		input.Placeholder = m.profiles[m.cursor].Name
	}
	input.Focus()

	m.input = input
	m.naming = true
	m.namingNew = creating
	m.err = ""
	return m
}

func (m ProfileListModel) updateNaming(msg tea.Msg) (ProfileListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "enter":
		return m.saveName(), nil
	case "esc":
		m.naming = false
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m ProfileListModel) saveName() ProfileListModel {
	name := strings.TrimSpace(m.input.Value())
	if name == "" && !m.namingNew {
		m.naming = false
		return m
	}
	if name == "" {
		m.err = domain.ProfileListNameRequired
		return m
	}
	for i, profile := range m.profiles {
		if profile.Name == name && (m.namingNew || i != m.cursor) {
			m.err = fmt.Sprintf(domain.ProfileListNameTakenFmt, name)
			return m
		}
	}

	if m.namingNew {
		m.profiles = append(m.profiles, domain.ProfileConfig{Name: name})
		m.cursor = len(m.profiles) - 1
	} else {
		m.profiles[m.cursor].Name = name
	}

	m.naming = false
	m.err = ""
	return m.ensureDefault()
}

func (m ProfileListModel) View() string {
	var b strings.Builder
	for i, profile := range m.profiles {
		label := profileLabel(profile)
		if m.naming && !m.namingNew && i == m.cursor {
			label = m.input.View()
		}
		if i == m.mark {
			label = domain.ProfileListMarkPrefix + label
		}
		m.renderRow(&b, label, i == m.cursor)
		b.WriteString("\n")
	}
	if m.naming && m.namingNew {
		m.renderRow(&b, m.input.View(), true)
		b.WriteString("\n")
	}
	m.renderRow(&b, domain.ProfileListDoneRow, m.cursor == m.doneRow())

	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err))
	}
	return b.String()
}

// helpHint is the wizard help bar for this step. Marking a row changes what the
// next keypress does, so the bar has to say so — the second half of a merge is
// not something to guess.
func (m ProfileListModel) helpHint() string {
	if m.naming {
		return domain.ProfileListNamingHelp
	}
	if m.mark != noMark {
		return fmt.Sprintf(domain.ProfileListMergeHint, m.profiles[m.mark].Name)
	}
	return domain.ProfileListHelp
}

func profileLabel(profile domain.ProfileConfig) string {
	label := fmt.Sprintf(domain.ProfileListEntryFmt, profile.Name, strings.Join(profile.Jobs, domain.ProfileListJobSep))
	if profile.Default {
		label += domain.ProfileListDefaultSuffix
	}
	return label
}

func (m ProfileListModel) renderRow(b *strings.Builder, label string, selected bool) {
	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(styles.Indent + label))
}
