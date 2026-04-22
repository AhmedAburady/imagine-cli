package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// CustomTheme returns theme colors (kept for compatibility)
func CustomTheme() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(DraculaYellow)
}
