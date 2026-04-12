package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// TextInputModel is a prompt-style text field with validation.
type TextInputModel struct {
	input    textinput.Model
	title    string
	desc     string
	validate func(string) error
	width    int
	done     bool
	aborted  bool
	err      error
}

// NewTextInput creates a TextInputModel with a title and description.
func NewTextInput(params NewTextInputParams) TextInputModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = styles.InputPrompt.Render("❯ ")
	ti.CharLimit = 256

	if params.Placeholder != "" {
		ti.Placeholder = params.Placeholder
	}

	m := TextInputModel{
		input:    ti,
		title:    params.Title,
		desc:     params.Description,
		validate: params.Validate,
		width:    80,
	}

	return m
}

// NewTextInputParams holds inputs for NewTextInput.
type NewTextInputParams struct {
	Title       string
	Description string
	Placeholder string
	Validate    func(string) error
}

// Done returns true after the user confirmed their input.
func (m TextInputModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m TextInputModel) Aborted() bool { return m.aborted }

// Value returns the current input text.
func (m TextInputModel) Value() string { return strings.TrimSpace(m.input.Value()) }

// SetWidth updates the available width.
func (m *TextInputModel) SetWidth(w int) {
	m.width = w
	m.input.Width = max(10, w-4)
}

// SetHeight is a no-op (satisfies Sizable).
func (m *TextInputModel) SetHeight(_ int) {}

// Init starts the cursor blink.
func (m TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key events.
func (m TextInputModel) Update(msg tea.Msg) (TextInputModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if m.validate != nil {
				if err := m.validate(m.input.Value()); err != nil {
					m.err = err
					return m, nil
				}
			}
			m.done = true
			return m, nil
		case "esc":
			m.aborted = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = nil
	return m, cmd
}

// View renders the prompt-style input.
func (m TextInputModel) View() string {
	var b strings.Builder

	b.WriteString(m.input.View())

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(styles.DangerText.Render("  " + m.err.Error()))
	}

	return b.String()
}
