package views

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AhmedAburady/banana-cli/config"
	"github.com/AhmedAburady/banana-cli/ui"
)

// APIKeySavedMsg is emitted when the API key is successfully saved
type APIKeySavedMsg struct {
	APIKey string
}

// APIKeyModel represents the API key input view
type APIKeyModel struct {
	input    textinput.Model
	errorMsg string
	width    int
	height   int
}

// NewAPIKeyModel creates a new API key input model
func NewAPIKeyModel() APIKeyModel {
	ti := textinput.New()
	ti.Placeholder = "Enter your Gemini API key..."
	ti.CharLimit = 100
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.DraculaPurple)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.DraculaForeground)
	ti.Focus()

	return APIKeyModel{
		input: ti,
	}
}

// Init initializes the API key model
func (m APIKeyModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the API key view
func (m APIKeyModel) Update(msg tea.Msg) (APIKeyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			apiKey := m.input.Value()
			if apiKey == "" {
				m.errorMsg = "API key cannot be empty"
				return m, nil
			}

			// Save the API key
			if err := config.SaveAPIKey(apiKey); err != nil {
				m.errorMsg = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}

			// Success - emit saved message
			return m, func() tea.Msg {
				return APIKeySavedMsg{APIKey: apiKey}
			}

		case "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the API key input section (without banner - main.go adds that)
func (m APIKeyModel) View() string {
	// API key input section
	labelStyle := lipgloss.NewStyle().Foreground(ui.DraculaCyan).Bold(true)
	inputLabel := labelStyle.Render("Gemini API Key")

	// Input field with border
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.DraculaPurple).
		Padding(0, 1).
		Width(54)
	inputField := inputStyle.Render(m.input.View())

	// Error message if any
	var errorLine string
	if m.errorMsg != "" {
		errorLine = ui.ErrorStyle.Render(m.errorMsg)
	}

	// Help text
	helpText := ui.SubtleStyle.Render("Get your free API key from: https://aistudio.google.com/app/apikey")
	controls := ui.HelpStyle.Render("enter: save • esc: quit")

	return lipgloss.JoinVertical(lipgloss.Center,
		inputLabel,
		"",
		inputField,
		"",
		errorLine,
		"",
		helpText,
		"",
		controls,
	)
}

// SetSize updates the view dimensions
func (m *APIKeyModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
