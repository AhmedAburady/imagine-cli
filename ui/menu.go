package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MenuChoice represents menu options
type MenuChoice int

const (
	GenerateImage MenuChoice = iota
	EditImage
)

// MenuSelectionMsg is emitted when a menu option is selected
type MenuSelectionMsg struct {
	Choice MenuChoice
}

// MenuModel represents the main menu
type MenuModel struct {
	choices []string
	cursor  int
	width   int
	height  int
	styles  MenuStyles
}

// MenuStyles holds styles for the menu
type MenuStyles struct {
	Window       lipgloss.Style
	Title        lipgloss.Style
	Item         lipgloss.Style
	SelectedItem lipgloss.Style
	Help         lipgloss.Style
	Subtle       lipgloss.Style
}

// NewMenuModel creates a new menu model
func NewMenuModel(styles MenuStyles) MenuModel {
	return MenuModel{
		choices: []string{
			"Generate Image",
			"Edit Image",
		},
		cursor: 0,
		styles: styles,
	}
}

// Init initializes the menu model
func (m MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the menu
func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Select):
			return m, func() tea.Msg {
				return MenuSelectionMsg{Choice: MenuChoice(m.cursor)}
			}
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View renders the menu
func (m MenuModel) View() string {
	// Find max width for consistent bar size
	maxWidth := 0
	for _, choice := range m.choices {
		if len(choice) > maxWidth {
			maxWidth = len(choice)
		}
	}
	barWidth := maxWidth + 4 // padding

	// Build menu items - each line centered
	var lines []string
	for i, choice := range m.choices {
		style := m.styles.Item.Width(barWidth)
		if i == m.cursor {
			style = m.styles.SelectedItem.Width(barWidth)
		}
		lines = append(lines, style.Render(choice))
	}

	// Join menu items
	menuBlock := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Help text
	help := m.styles.Help.Render("\u2191/\u2193: navigate \u2022 enter: select \u2022 q: quit")

	// Combine content - center everything
	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		menuBlock,
		"",
		help,
	)

	return content
}

// SetSize updates the menu dimensions
func (m *MenuModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Key bindings
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("\u2191/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("\u2193/j", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
