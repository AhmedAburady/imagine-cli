package commands

import (
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// Shared CLI styling primitives, used by the generation UI and the metadata reader.
var (
	uiDim    = lipgloss.Color("#6C7086")
	uiCyan   = lipgloss.Color("#00B2C7")
	uiYellow = lipgloss.Color("#F2C94C")
	uiInk    = lipgloss.Color("#1A1A1A")

	uiPale      = lipgloss.NewStyle().Foreground(uiDim)
	uiPill      = lipgloss.NewStyle().Foreground(uiInk).Background(uiYellow).Bold(true).Padding(0, 1)
	uiCyanStyle = lipgloss.NewStyle().Foreground(uiCyan)
	uiHeader    = lipgloss.NewStyle().Foreground(uiCyan).Bold(true)
	uiAmber     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

// stdoutIsTTY reports whether stdout is an interactive terminal.
func stdoutIsTTY() bool { return term.IsTerminal(os.Stdout.Fd()) }

// paint applies a style only on a TTY, leaving piped output free of escape codes.
func paint(st lipgloss.Style, s string) string {
	if stdoutIsTTY() {
		return st.Render(s)
	}
	return s
}

// warnLine renders a non-blocking [WARNING] notice in the amber tag style.
func warnLine(msg string) string {
	return " " + paint(uiAmber, "[WARNING]") + "  " + paint(uiPale, msg)
}
