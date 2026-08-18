package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// modalPadding is the horizontal padding of the modal box, modalMargin the rows
// it leaves to the frame underneath so a question never reads as a new page.
const (
	modalPadding = 2
	modalMargin  = 4
)

// marker registers a clickable region on rendered content. The zone manager
// implements it; a test can pass its own.
type marker interface {
	Mark(id, content string) string
}

// modalShape is how a session is laid out, not what it asks: a stepper walks the
// questions one at a time like the CLI wizard, a form puts a whole session on
// screen at once for a decision the user takes in one go.
type modalShape int

const (
	modalStepper modalShape = iota
	modalForm
)

// promptReply carries the session's answers back to the flow goroutine blocked in
// Prompter.Ask.
type promptReply struct {
	answers flow.Answers
	err     error
}

type modalParams struct {
	Title   string
	Shape   modalShape
	Session flow.Session
	Reply   chan<- promptReply
	Width   int
	Height  int
}

type modal struct {
	open    bool
	shape   modalShape
	title   string
	session flow.Session
	answers flow.Answers
	reply   chan<- promptReply

	width  int
	height int

	index   int
	kind    flow.StepKind
	content flow.StepContent
	text    components.TextInputModel
	list    components.SelectListModel

	rows   []formRow
	focus  int
	chosen flow.Answers
	// lifted holds the refusals the user acknowledged, keyed by blocker, so a
	// rebuild of the rows never silently un-ticks one.
	lifted     map[string]bool
	generation int

	loading string
}

type modalLoadedMsg struct {
	index   int
	content flow.StepContent
	err     error
}

func newModal(params modalParams) (modal, tea.Cmd) {
	mo := modal{
		open:    true,
		shape:   params.Shape,
		title:   params.Title,
		session: params.Session,
		answers: params.Session.Presets,
		chosen:  params.Session.Presets,
		reply:   params.Reply,
		width:   params.Width,
		height:  params.Height,
		index:   -1,
	}
	if params.Shape == modalForm {
		mo.loading = domain.DashboardModalPreparing
		return mo, buildFormCmd(mo.session, mo.chosen, mo.generation)
	}
	return mo.advance()
}

func (mo modal) resize(width, height int) modal {
	mo.width, mo.height = width, height
	mo.list.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
	mo.text.SetWidth(mo.bodyWidth())
	return mo
}

func (mo modal) bodyWidth() int {
	return max(rules.ModalWidth(mo.width)-domain.DashboardModalChrome-modalPadding, 1)
}

// bodyHeight leaves the surrounding frame visible: a modal that filled the screen
// would read as a new page rather than as a question over the dashboard.
func (mo modal) bodyHeight() int {
	return max(mo.height-domain.DashboardModalChrome-modalMargin, 1)
}

// answerable steps are the ones this surface has to ask: a preset is already
// answered and a skipped one is irrelevant, both without a question.
func (mo modal) advance() (modal, tea.Cmd) {
	for index := mo.index + 1; index < len(mo.session.Steps); index++ {
		step := mo.session.Steps[index]
		if _, known := mo.answers.Get(step.Key); known {
			continue
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(mo.answers); skip {
				mo.answers = mo.answers.With(step.Key, flow.Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		mo.index = index
		return mo.enter(step)
	}
	return mo.submit(mo.answers)
}

func (mo modal) enter(step flow.Step) (modal, tea.Cmd) {
	mo.kind = step.Kind
	if step.Load != nil {
		mo.loading = loadingMessage(step)
		return mo, loadStepCmd(mo.index, step, mo.answers)
	}
	content, err := stepContent(step, mo.answers)
	if err != nil {
		return mo.fail(err)
	}
	return mo.show(step, content)
}

func (mo modal) show(step flow.Step, content flow.StepContent) (modal, tea.Cmd) {
	mo.loading, mo.content = "", content
	switch step.Kind {
	case flow.StepText:
		mo.text = components.NewTextInput(components.NewTextInputParams{
			Title:       content.Title,
			Description: content.Description,
			Validate:    step.Validate,
		})
		mo.text.SetWidth(mo.bodyWidth())
		return mo, mo.text.Init()
	case flow.StepSelect, flow.StepRecap:
		mo.list = newSelectList(content, step.Kind == flow.StepRecap)
	case flow.StepBranchSelect:
		mo.list = components.NewSelectList(components.NewSelectListParams{
			Title:       content.Title,
			Description: content.Description,
			Items:       branchItems(step),
		})
	default:
		return mo.fail(fmt.Errorf(domain.DashboardUnsupportedStepFmt, step.Key, step.Kind))
	}
	mo.list.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
	return mo, nil
}

// back returns to the previous question. A step answered from a preset or skipped
// was never asked, so it is not a place the user can go back to.
func (mo modal) back() (modal, tea.Cmd) {
	for index := mo.index - 1; index >= 0; index-- {
		step := mo.session.Steps[index]
		if answer, known := mo.answers.Get(step.Key); !known || !answer.Asked {
			continue
		}
		mo.index = index
		return mo.enter(step)
	}
	return mo.cancel()
}

func (mo modal) submit(answers flow.Answers) (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{answers: answers})
}

func (mo modal) cancel() (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{err: domain.ErrUserAborted})
}

func (mo modal) fail(err error) (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{err: err})
}

func (mo modal) close() modal { return modal{} }

func (mo modal) update(msg tea.Msg) (modal, tea.Cmd) {
	switch msg := msg.(type) {
	case modalLoadedMsg:
		return mo.applyLoaded(msg)
	case formReadyMsg:
		return mo.applyForm(msg)
	case tea.KeyMsg:
		// A modal waiting on I/O has nothing to answer with yet, and the flow it
		// would answer is busy producing it.
		if mo.loading != "" {
			return mo, nil
		}
		if mo.shape == modalForm {
			return mo.updateForm(msg)
		}
		return mo.updateStepper(msg)
	}
	return mo, nil
}

func (mo modal) applyLoaded(msg modalLoadedMsg) (modal, tea.Cmd) {
	if msg.index != mo.index {
		return mo, nil
	}
	if msg.err != nil {
		return mo.fail(msg.err)
	}
	return mo.show(mo.session.Steps[msg.index], msg.content)
}

func (mo modal) updateStepper(msg tea.KeyMsg) (modal, tea.Cmd) {
	if mo.kind == flow.StepText {
		var cmd tea.Cmd
		mo.text, cmd = mo.text.Update(msg)
		switch {
		case mo.text.Aborted():
			return mo.back()
		case mo.text.Done():
			return mo.answer(mo.text.Value())
		}
		return mo, cmd
	}

	var cmd tea.Cmd
	mo.list, cmd = mo.list.Update(msg)
	switch {
	case mo.list.Aborted():
		return mo.back()
	case mo.list.Chosen():
		if value := mo.list.Value(); value != domain.WizardCancelValue {
			return mo.answer(value)
		}
		return mo.cancel()
	}
	return mo, cmd
}

func (mo modal) answer(value string) (modal, tea.Cmd) {
	mo.answers = mo.answers.With(mo.session.Steps[mo.index].Key, flow.Answer{Value: value, Asked: true})
	return mo.advance()
}

func newSelectList(content flow.StepContent, recap bool) components.SelectListModel {
	items := make([]components.SelectItem, 0, len(content.Options)+2)
	for _, option := range content.Options {
		items = append(items, components.SelectItem{
			Label:     option.Label,
			Value:     option.Value,
			Separator: option.Separator,
			Danger:    option.Danger,
		})
	}
	if recap {
		items = append(items,
			components.SelectItem{Separator: true},
			components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
		)
	}
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
	})
}

func branchItems(step flow.Step) []components.SelectItem {
	pinned := ""
	for _, candidate := range step.Branches {
		if candidate.Name == step.Pinned {
			pinned = step.Pinned
			break
		}
	}
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   step.Branches,
		Pinned:       pinned,
		PinnedSuffix: domain.PinnedSuffixDefault,
	})
}

// stepContent merges what a step declares statically with what it derives from
// the answers, so a Build only returns the parts that change.
func stepContent(step flow.Step, answers flow.Answers) (flow.StepContent, error) {
	content := flow.StepContent{Title: step.Title, Description: step.Description, Options: step.Options}
	build := step.Build
	if build == nil {
		build = step.Load
	}
	if build == nil {
		return content, nil
	}
	built, err := build(answers)
	if err != nil {
		return flow.StepContent{}, err
	}
	if built.Title != "" {
		content.Title = built.Title
	}
	if built.Description != "" {
		content.Description = built.Description
	}
	if len(built.Options) > 0 {
		content.Options = built.Options
	}
	content.Blockers = built.Blockers
	return content, nil
}

func loadingMessage(step flow.Step) string {
	if step.LoadingMessage != "" {
		return step.LoadingMessage
	}
	return domain.DashboardModalPreparing
}

func loadStepCmd(index int, step flow.Step, answers flow.Answers) tea.Cmd {
	return func() tea.Msg {
		content, err := step.Load(answers)
		return modalLoadedMsg{index: index, content: content, err: err}
	}
}

func replyCmd(reply chan<- promptReply, answer promptReply) tea.Cmd {
	return func() tea.Msg {
		reply <- answer
		return nil
	}
}

func (mo modal) view(zones marker) string {
	body := mo.body(zones)
	rect := rules.ComputeModalRect(rules.ModalRectParams{
		ScreenWidth:   mo.width,
		ScreenHeight:  mo.height,
		ContentHeight: len(body),
	})
	if rect.Width <= domain.DashboardModalChrome || rect.Height <= domain.DashboardModalChrome {
		return ""
	}

	lines := body
	if height := rect.Height - domain.DashboardModalChrome; len(lines) > height {
		lines = lines[:height]
	}
	box := styles.DashboardModal.
		Width(rect.Width - domain.DashboardModalChrome).
		Render(strings.Join(lines, "\n"))

	return lipgloss.NewStyle().MarginLeft(rect.X).MarginTop(rect.Y).Render(box)
}

func (mo modal) body(zones marker) []string {
	lines := []string{styles.DashboardModalTitle.Render(truncate(mo.title, mo.bodyWidth())), ""}
	switch {
	case mo.loading != "":
		lines = append(lines, styles.DashboardEmpty.Render(truncate(mo.loading, mo.bodyWidth())))
	case mo.shape == modalForm:
		lines = append(lines, mo.formBody(zones)...)
	case mo.kind == flow.StepText:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.text.View(), "\n")...)
	default:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.list.View(), "\n")...)
	}
	return append(lines, "", styles.DashboardModalHint.Render(truncate(mo.hint(), mo.bodyWidth())))
}

// stepHeader carries what the widgets do not draw themselves: the question and
// what it entails. The CLI wizard renders the same two around its own steps.
func (mo modal) stepHeader() []string {
	var lines []string
	if mo.content.Title != "" && mo.content.Title != mo.title {
		lines = append(lines, styles.Bold.Render(truncate(mo.content.Title, mo.bodyWidth())))
	}
	for _, line := range strings.Split(mo.content.Description, "\n") {
		if mo.content.Description == "" {
			break
		}
		lines = append(lines, styles.Muted.Render(truncate(line, mo.bodyWidth())))
	}
	if len(lines) == 0 {
		return nil
	}
	return append(lines, "")
}

func (mo modal) hint() string {
	if mo.shape == modalForm {
		return domain.DashboardFormHint
	}
	return domain.DashboardStepperHint
}
