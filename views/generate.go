package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AhmedAburady/banana-cli/ui"
)

// GenerateSubmitMsg is emitted when the generate form is submitted
type GenerateSubmitMsg struct {
	ui.GenerateFormData
}

// GenerateModel represents the generate image view
type GenerateModel struct {
	form     *ui.Form
	styles   ui.FormStyles
	errorMsg string
	width    int
	height   int
}

// NewGenerateModel creates a new generate model
func NewGenerateModel() GenerateModel {
	form := ui.BuildForm(ui.GenerateFormConfig())
	// Thinking level and Image Search are Flash-only; Pro is default, so hide initially
	form.SetFieldHidden("thinking", true)
	form.SetFieldHidden("imagesearch", true)
	return GenerateModel{
		form:   form,
		styles: ui.DefaultFormStyles(),
	}
}

// Init initializes the generate model
func (m GenerateModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages for the generate view
func (m GenerateModel) Update(msg tea.Msg) (GenerateModel, tea.Cmd) {
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

func (m *GenerateModel) submit() tea.Cmd {
	data, err := ui.ValidateGenerateForm(m.form)
	if err != nil {
		m.errorMsg = err.Error()
		m.form.Reset()
		return nil
	}

	return func() tea.Msg { return GenerateSubmitMsg{data} }
}

// View renders the generate view
func (m GenerateModel) View() string {
	title := m.styles.Title.Render("Generate Image")

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
func (m *GenerateModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
