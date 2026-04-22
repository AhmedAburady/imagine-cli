package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AhmedAburady/banana-cli/ui"
)

// EditSubmitMsg is emitted when the edit form is submitted
type EditSubmitMsg struct {
	ui.EditFormData
}

// EditModel represents the edit image view
type EditModel struct {
	form     *ui.Form
	styles   ui.FormStyles
	errorMsg string
	width    int
	height   int
}

// Default prompt for editing
const defaultEditPrompt = "A 2D vector art pattern in the style of the reference image/s not a copy of it not an imitation not an edit of it rather, a pattern inspired by the shapes in reference image/s you can get creative with colors and avoid extremely bold outlines"

// NewEditModel creates a new edit model
func NewEditModel() EditModel {
	form := ui.BuildForm(ui.EditFormConfig())
	// Thinking level and Image Search are Flash-only; Pro is default, so hide initially
	form.SetFieldHidden("thinking", true)
	form.SetFieldHidden("imagesearch", true)
	return EditModel{
		form:   form,
		styles: ui.DefaultFormStyles(),
	}
}

// Init initializes the edit model
func (m EditModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages for the edit view
func (m EditModel) Update(msg tea.Msg) (EditModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return ui.BackToMenuMsg{} }
		case "ctrl+s":
			return m, m.submit()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	cmd := m.form.Update(msg)

	// Thinking level and Image Search are Flash-only features
	isPro := m.form.GetString("model") != "flash"
	m.form.SetFieldHidden("thinking", isPro)
	m.form.SetFieldHidden("imagesearch", isPro)

	if m.form.Submitted() {
		return m, m.submit()
	}

	return m, cmd
}

func (m *EditModel) submit() tea.Cmd {
	data, err := ui.ValidateEditForm(m.form, defaultEditPrompt)
	if err != nil {
		m.errorMsg = err.Error()
		m.form.Reset()
		return nil
	}

	return func() tea.Msg { return EditSubmitMsg{data} }
}

// View renders the edit view
func (m EditModel) View() string {
	title := m.styles.Title.Render("Edit Image")

	var errorLine string
	if m.errorMsg != "" {
		errorLine = ui.ErrorStyle.Render("Error: " + m.errorMsg)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		m.form.View(),
		"",
		errorLine,
	)

	return content
}

// SetSize updates the view dimensions
func (m *EditModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
