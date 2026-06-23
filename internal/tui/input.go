package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputModel is a single-line text input prompt. width/height track the terminal size so
// View can center the dialog as a modal.
type InputModel struct {
	prompt    string
	cancelled bool
	submitted bool
	err       error
	input     textinput.Model
	width     int
	height    int
}

// NewInput creates an input model.
func NewInput(prompt string) *InputModel {
	ti := textinput.New()
	ti.Placeholder = "type here"
	ti.Focus()
	return &InputModel{
		prompt: prompt,
		input:  ti,
	}
}

// Value returns the entered text.
func (m *InputModel) Value() string {
	return m.input.Value()
}

// Submitted reports whether the user pressed Enter.
func (m *InputModel) Submitted() bool {
	return m.submitted
}

// Cancelled reports whether the user cancelled.
func (m *InputModel) Cancelled() bool {
	return m.cancelled
}

// Init implements tea.Model.
func (m *InputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.submitted = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Size the field to a modal width: a comfortable fixed width, but never wider than
		// the terminal can hold inside the dialog's border + padding.
		w := 40
		if max := msg.Width - 8; max < w {
			w = max
		}
		if w < 10 {
			w = 10
		}
		m.input.Width = w
	case error:
		m.err = msg
		return m, tea.Quit
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model. Standalone it centers the dialog box in the terminal;
// embedded in the dashboard the caller draws Box() and composites it over the live tree.
func (m *InputModel) View() string {
	box := m.Box()
	if m.width <= 0 || m.height <= 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// Box renders the prompt as a bordered dialog without centering, for the dashboard overlay.
func (m *InputModel) Box() string {
	if m.err != nil {
		return dialogStyle.Render(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	content := fmt.Sprintf("%s\n\n%s\n\n%s",
		titleStyle.Render(m.prompt),
		m.input.View(),
		helpStyle.Render("enter to confirm · esc to cancel"))
	return dialogStyle.Render(content)
}

var _ tea.Model = &InputModel{}
