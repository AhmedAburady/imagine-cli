package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Dracula Color Palette
var (
	DraculaBackground = lipgloss.Color("#282a36")
	DraculaForeground = lipgloss.Color("#f8f8f2")
	DraculaCyan       = lipgloss.Color("#8be9fd")
	DraculaGreen      = lipgloss.Color("#50fa7b")
	DraculaPink       = lipgloss.Color("#ff79c6")
	DraculaPurple     = lipgloss.Color("#bd93f9")
	DraculaRed        = lipgloss.Color("#ff5555")
	DraculaYellow     = lipgloss.Color("#FFFF00")
	DraculaOrange     = lipgloss.Color("#ffb86c")
)

// Window/Box Styles with Rounded Corners
var (
	WindowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DraculaPurple).
			Padding(1, 2).
			Margin(1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(DraculaYellow).
			MarginBottom(1)

	MenuItemStyle = lipgloss.NewStyle().
			Foreground(DraculaForeground).
			PaddingLeft(2).
			PaddingRight(2)

	MenuSelectedStyle = lipgloss.NewStyle().
				Foreground(DraculaBackground).
				Background(DraculaPurple).
				Bold(true).
				PaddingLeft(2).
				PaddingRight(2)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(DraculaRed).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(DraculaGreen).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(DraculaCyan)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))
)
