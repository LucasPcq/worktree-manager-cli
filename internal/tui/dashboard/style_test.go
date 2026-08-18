package dashboard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/styles"
)

func TestAWorktreeRowIsTwoLinesAndSaysWhatItsStateIs(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.statuses[1] = domain.WorktreeStatus{Branch: "b", Path: "/tmp/b", IsDirty: true, CommitsAhead: 3}
	model.parents = map[string]string{"b": "main"}
	model = update(model, prsMsg{prs: []domain.PRInfo{{Number: 61, Branch: "b"}}})

	lines := model.renderRow(1, model.layout().List.Width-borderWidth-paddingWidth)

	if len(lines) != domain.DashboardRowHeight {
		t.Fatalf("a row is %d lines, want %d", len(lines), domain.DashboardRowHeight)
	}
	if !strings.Contains(lines[0], "b") {
		t.Error("the first line names the worktree")
	}
	for _, want := range []string{"main", "#61", "base"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("the second line is missing %q — that is what it is there for:\n%s", want, lines[1])
		}
	}
	if lipgloss.Width(lines[0]) != lipgloss.Width(lines[1]) {
		t.Errorf("the two lines are %d and %d wide; a row is one block and its zone covers both",
			lipgloss.Width(lines[0]), lipgloss.Width(lines[1]))
	}
}

func TestTheSelectedRowIsTintedAcrossItsWholeWidth(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	width := model.layout().List.Width - borderWidth - paddingWidth

	selected := model.renderRow(0, width)
	plain := model.renderRow(1, width)

	if !strings.Contains(selected[0], rowBar) {
		t.Error("the selected row carries the accent bar")
	}
	if strings.Contains(plain[0], rowBar) {
		t.Error("only the selected row carries the bar")
	}
	if selected[0] == plain[0] {
		t.Error("the selection must be visible on the row itself, not only through the detail panel")
	}
	for _, line := range append(selected, plain...) {
		if lipgloss.Width(line) != width {
			t.Errorf("row line is %d wide, want the full %d so the block reads as one", lipgloss.Width(line), width)
		}
	}
}

func TestTheHeaderNamesTheProductAndUnderlinesTheActiveTab(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	header := model.renderHeader(model.layout())
	lines := strings.Split(header, "\n")

	if len(lines) != domain.DashboardHeaderHeight {
		t.Fatalf("the header is %d lines, want %d", len(lines), domain.DashboardHeaderHeight)
	}
	if !strings.Contains(lines[0], domain.DashboardWordmark) || !strings.Contains(lines[0], domain.DashboardTabWorktrees) {
		t.Errorf("header = %q, want the wordmark and the tabs", lines[0])
	}
	if !strings.Contains(lines[0], "2 worktrees") {
		t.Errorf("header = %q, want the count of what is listed", lines[0])
	}
	if !strings.Contains(lines[1], domain.DashboardActiveRuleGlyph) {
		t.Errorf("rule = %q, want the active tab underlined rather than filled", lines[1])
	}
}

func TestTheHeaderCountAgreesWithTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "only")

	if got := model.renderHeader(model.layout()); !strings.Contains(got, "1 worktree ") {
		t.Errorf("header = %q, want a single worktree named in the singular", got)
	}
}

// Points 3 of the style pass: the two confirmations are the same widget, so they
// cannot drift apart again.
func TestBothConfirmationsAreDrawnWithTheSameButtons(t *testing.T) {
	create, _ := openStepper(t, stepperSession())
	create = typeText(t, create, "feature/new")
	create = press(t, create, namedKey(13))
	create = press(t, create, namedKey(13))

	remove, _ := openDeleteForm(t, deleteSession())

	createButtons := buttonLabels(create)
	removeButtons := buttonLabels(remove)
	if len(createButtons) != 2 || len(removeButtons) != 2 {
		t.Fatalf("buttons: create %v, delete %v — both confirm and cancel", createButtons, removeButtons)
	}
	if createButtons[1] != removeButtons[1] {
		t.Errorf("cancel reads %q on a create and %q on a delete", createButtons[1], removeButtons[1])
	}
	if create.modal.renderButton(0, formRow{label: "X", confirm: true}) != remove.modal.renderButton(0, formRow{label: "X", confirm: true}) {
		t.Error("the same button must render the same way in both modals")
	}
}

func buttonLabels(model Model) []string {
	var labels []string
	for _, row := range model.modal.rows {
		if row.kind == formButton {
			labels = append(labels, row.label)
		}
	}
	return labels
}

func TestARecapStepIsAConfirmationNotAnotherList(t *testing.T) {
	model, _ := openStepper(t, stepperSession())
	model = typeText(t, model, "feature/new")
	model = press(t, model, namedKey(13))
	model = press(t, model, namedKey(13))

	if !model.modal.usesRows() {
		t.Fatal("the recap must be drawn as a confirmation")
	}
	if got := model.modal.hint(); got != domain.DashboardStepperRowsHint {
		t.Errorf("hint = %q, want the keys a stepper confirmation takes", got)
	}
	if strings.Contains(strings.Join(model.modal.body(m0zones{}), "\n"), domain.WizardCancelValue) {
		t.Error("the cancel row must read as a button, not carry its wizard value")
	}
}

func TestAPlainConfirmSaysNothingAboutToggling(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepRecap, Key: keyConfirm, Title: "Sure?",
		Options: []flow.Option{{Label: "Yes", Value: confirmYes}},
	}}}
	model := newTestModel(t, testWidth, testHeight, "a")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{shape: modalForm, session: session, reply: reply})

	if got := model.modal.hint(); got != domain.DashboardConfirmHint {
		t.Errorf("hint = %q, want no mention of toggling in a form with nothing to toggle", got)
	}
}

func TestTheRowMetaStartsWhereTheNameDoes(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.parents = map[string]string{"b": "main"}
	width := model.layout().List.Width - borderWidth - paddingWidth

	for index, name := range []string{"a", "b"} {
		lines := model.renderRow(index, width)
		if indentOf(lines[0]) != indentOf(lines[1]) {
			t.Errorf("%s: the name starts at column %d and the line under it at %d — they must align",
				name, indentOf(lines[0]), indentOf(lines[1]))
		}
	}
}

// indentOf counts the leading blanks of a rendered line, styling aside.
func indentOf(line string) int {
	plain := stripANSI(line)
	return len(plain) - len(strings.TrimLeft(plain, " ▌"))
}

func stripANSI(text string) string {
	var out strings.Builder
	inEscape := false
	for _, char := range text {
		switch {
		case char == '\x1b':
			inEscape = true
		case inEscape && (char == 'm' || char == 'z'):
			inEscape = false
		case !inEscape:
			out.WriteRune(char)
		}
	}
	return out.String()
}

func TestTheContextMenuNamesItsWorktreeAndRulesItOff(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feature/x")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))

	box, rect := model.menuBox()
	lines := strings.Split(box, "\n")

	if !strings.Contains(box, "feature/x") {
		t.Error("the menu must name the worktree it acts on")
	}
	if !strings.Contains(box, domain.DashboardRuleGlyph) {
		t.Error("the actions must be ruled off from that name")
	}
	// Everything in the box hangs off the same left edge: the name, the rule and
	// the entries, with the focused one marked by the tint alone.
	for index, line := range lines[1:3] {
		if indentOf(line) != 0 {
			t.Errorf("menu line %d starts at column %d, want it flush left", index+1, indentOf(line))
		}
	}
	if rect.Height != len(lines) {
		t.Errorf("the placement rule reserves %d rows for a box of %d", rect.Height, len(lines))
	}
}

func TestOnlyTheHeaderButtonIsAFilledBlock(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	form, _ := openDeleteForm(t, deleteSession())

	button := form.modal.renderButton(99, formRow{label: "Yes, delete", confirm: true})

	if !strings.Contains(stripANSI(button), "[ Yes, delete ]") {
		t.Errorf("an unfocused action reads by its brackets, not by a slab: %q", stripANSI(button))
	}
	if strings.Contains(button, styles.DashboardAddButton.Render("")) && styles.DashboardAddButton.Render("") != "" {
		t.Error("the filled block belongs to the header button alone")
	}
	if !strings.Contains(model.addButton(model.layout()), domain.DashboardAddLabelLong) {
		t.Error("the header button says what it does when there is room for it")
	}
}

func TestTheDetailIsGroupedUnderHeadings(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.statuses[0] = domain.WorktreeStatus{Branch: "a", Path: "/tmp/a", IsDirty: true, CommitsAhead: 2}
	model = update(model, prsMsg{})

	body := model.detailBody(model.layout())
	joined := strings.Join(body, "\n")

	for _, heading := range []string{
		domain.DashboardSectionWorktree, domain.DashboardSectionDivergence, domain.DashboardSectionReview,
	} {
		index := lineIndex(body, heading)
		if index < 0 {
			t.Fatalf("the detail is missing the %q group", heading)
		}
		if body[index-1] != "" || body[index+1] != "" {
			t.Errorf("%q must stand on its own, with a blank line either side", heading)
		}
	}
	if !strings.Contains(body[0], "a") || !strings.Contains(body[0], "dirty") {
		t.Errorf("the heading = %q, want the worktree and its state", body[0])
	}
	if !strings.Contains(body[1], domain.DashboardRuleGlyph) {
		t.Error("a rule must separate the heading from the fields, as in the context menu")
	}
	if strings.Contains(joined, domain.DashboardLabelState) {
		t.Error("the working-tree state is the pill in the heading; repeating it as a field says it twice")
	}
}

func lineIndex(lines []string, needle string) int {
	for index, line := range lines {
		if strings.Contains(line, needle) {
			return index
		}
	}
	return -1
}

func TestTheParentWorktreeCannotBeDeletedAndTheMenuSaysSo(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feature/x")
	model.statuses[0] = domain.WorktreeStatus{Branch: "main", Path: "/tmp/main", IsParent: true}

	item := model.menuItems()[0]
	if item.disabled == "" {
		t.Fatal("the parent worktree is refused whatever happens; the entry must say so up front")
	}
	if !strings.Contains(item.disabled, "parent") {
		t.Errorf("reason = %q, want it to name why", item.disabled)
	}

	model = update(model, key(domain.KeyMenu))
	model, cmd := updateCmd(model, namedKey(13))

	if cmd != nil || len(model.ops.running) != 0 {
		t.Errorf("running = %+v, want the disabled entry to have started nothing", model.ops.running)
	}
}
