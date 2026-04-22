package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AhmedAburady/banana-cli/api"
)

// Internal colors for form styling
var (
	colorSelection = lipgloss.Color("#44475a")
	colorComment   = lipgloss.Color("#6272a4")
)

// Form styles
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(DraculaCyan).
			Bold(true).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(DraculaPurple).
			Bold(true)

	descStyle = lipgloss.NewStyle().
			Foreground(colorComment).
			Italic(true)

	focusedStyle = lipgloss.NewStyle().
			Foreground(DraculaYellow)

	blurredStyle = lipgloss.NewStyle().
			Foreground(DraculaForeground)

	cursorStyle = lipgloss.NewStyle().
			Foreground(DraculaPink)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorComment)
)

// FieldType represents different field types
type FieldType int

const (
	FieldInput FieldType = iota
	FieldTextArea
	FieldSelect
	FieldToggle
	FieldPath
)

// SelectOption represents an option in a select field
type SelectOption struct {
	Label string
	Value string
}

// FormField represents a single form field
type FormField struct {
	Type           FieldType
	Key            string
	Label          string
	Description    string
	Placeholder    string
	Value          string
	BoolValue      bool
	Options        []SelectOption
	Selected       int
	DirsOnly       bool
	AllowedExts    []string
	Hidden         bool
	InlineWithPrev bool // Render on the same row as the previous visible field

	// Internal components
	textInput textinput.Model
	textArea  textarea.Model
}

// Form represents a complete form with multiple fields
type Form struct {
	Title      string
	Fields     []FormField
	FocusIndex int
	Width      int
	submitted  bool
	errorMsg   string
}

// NewForm creates a new form
func NewForm(title string) *Form {
	return &Form{
		Title: title,
		Width: 60,
	}
}

// AddInput adds a text input field
func (f *Form) AddInput(key, label, description, placeholder, defaultValue string) *Form {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Width = f.Width - 4
	ti.CharLimit = 256
	ti.SetValue(defaultValue)

	field := FormField{
		Type:        FieldInput,
		Key:         key,
		Label:       label,
		Description: description,
		Placeholder: placeholder,
		Value:       defaultValue,
		textInput:   ti,
	}
	f.Fields = append(f.Fields, field)
	return f
}

// AddTextArea adds a multi-line text area field
func (f *Form) AddTextArea(key, label, description, placeholder string, lines int) *Form {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.SetWidth(f.Width - 4)
	ta.SetHeight(lines)
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	field := FormField{
		Type:        FieldTextArea,
		Key:         key,
		Label:       label,
		Description: description,
		Placeholder: placeholder,
		textArea:    ta,
	}
	f.Fields = append(f.Fields, field)
	return f
}

// AddSelect adds a select field with options
func (f *Form) AddSelect(key, label, description string, options []SelectOption, defaultIdx int) *Form {
	field := FormField{
		Type:        FieldSelect,
		Key:         key,
		Label:       label,
		Description: description,
		Options:     options,
		Selected:    defaultIdx,
	}
	if defaultIdx >= 0 && defaultIdx < len(options) {
		field.Value = options[defaultIdx].Value
	}
	f.Fields = append(f.Fields, field)
	return f
}

// AddToggle adds a toggle/boolean field
func (f *Form) AddToggle(key, label, description string, defaultValue bool) *Form {
	field := FormField{
		Type:        FieldToggle,
		Key:         key,
		Label:       label,
		Description: description,
		BoolValue:   defaultValue,
	}
	f.Fields = append(f.Fields, field)
	return f
}

// AddPath adds a path input with autocomplete
func (f *Form) AddPath(key, label, description, placeholder, defaultValue string, dirsOnly bool, allowedExts []string) *Form {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Width = f.Width - 4
	ti.CharLimit = 512
	ti.ShowSuggestions = true
	ti.SetValue(defaultValue)

	field := FormField{
		Type:        FieldPath,
		Key:         key,
		Label:       label,
		Description: description,
		Placeholder: placeholder,
		Value:       defaultValue,
		DirsOnly:    dirsOnly,
		AllowedExts: allowedExts,
		textInput:   ti,
	}

	// Set initial suggestions
	suggestions := computePathSuggestions(defaultValue, dirsOnly, allowedExts)
	ti.SetSuggestions(suggestions)
	field.textInput = ti

	f.Fields = append(f.Fields, field)
	return f
}

// Init initializes the form
func (f *Form) Init() tea.Cmd {
	if len(f.Fields) > 0 {
		// Focus the first visible field
		idx := 0
		if f.Fields[0].Hidden {
			idx = f.nextVisibleField(0)
		}
		f.FocusIndex = idx
		f.focusField(idx)
	}
	return textinput.Blink
}

// focusField focuses a specific field
func (f *Form) focusField(idx int) {
	for i := range f.Fields {
		field := &f.Fields[i]
		if i == idx {
			switch field.Type {
			case FieldInput, FieldPath:
				field.textInput.Focus()
				field.textInput.PromptStyle = focusedStyle
				field.textInput.TextStyle = focusedStyle
				field.textInput.Cursor.Style = cursorStyle
			case FieldTextArea:
				field.textArea.Focus()
			}
		} else {
			switch field.Type {
			case FieldInput, FieldPath:
				field.textInput.Blur()
				field.textInput.PromptStyle = blurredStyle
				field.textInput.TextStyle = blurredStyle
			case FieldTextArea:
				field.textArea.Blur()
			}
		}
	}
}

// Update handles form updates
func (f *Form) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "shift+tab":
			f.FocusIndex = f.prevVisibleField(f.FocusIndex)
			f.focusField(f.FocusIndex)
			return textinput.Blink

		case "down":
			f.FocusIndex = f.nextVisibleField(f.FocusIndex)
			f.focusField(f.FocusIndex)
			return textinput.Blink

		case "tab":
			// For path fields, check if there's a suggestion to accept
			if f.FocusIndex < len(f.Fields) {
				field := &f.Fields[f.FocusIndex]
				if field.Type == FieldPath {
					// Let textinput handle tab for autocomplete
					var cmd tea.Cmd
					field.textInput, cmd = field.textInput.Update(msg)
					field.Value = field.textInput.Value()
					// Update suggestions after tab
					suggestions := computePathSuggestions(field.Value, field.DirsOnly, field.AllowedExts)
					field.textInput.SetSuggestions(suggestions)
					return cmd
				}
			}
			// Move to next visible field
			f.FocusIndex = f.nextVisibleField(f.FocusIndex)
			f.focusField(f.FocusIndex)
			return textinput.Blink

		case "alt+enter", "ctrl+n":
			// Insert newline in textarea
			if f.FocusIndex < len(f.Fields) {
				field := &f.Fields[f.FocusIndex]
				if field.Type == FieldTextArea {
					field.textArea.InsertString("\n")
					field.Value = field.textArea.Value()
					return nil
				}
			}

		case "enter":
			// Move to next visible field or submit (even for textarea)
			next := f.nextVisibleField(f.FocusIndex)
			if next <= f.FocusIndex {
				// Wrapped around — we've passed the last field, submit
				f.submitted = true
				return nil
			}
			f.FocusIndex = next
			f.focusField(f.FocusIndex)
			return textinput.Blink

		case "left", "h":
			if f.FocusIndex < len(f.Fields) {
				field := &f.Fields[f.FocusIndex]
				if field.Type == FieldSelect {
					field.Selected--
					if field.Selected < 0 {
						field.Selected = len(field.Options) - 1
					}
					field.Value = field.Options[field.Selected].Value
					return nil
				}
				if field.Type == FieldToggle {
					field.BoolValue = !field.BoolValue
					return nil
				}
			}

		case "right", "l":
			if f.FocusIndex < len(f.Fields) {
				field := &f.Fields[f.FocusIndex]
				if field.Type == FieldSelect {
					field.Selected++
					if field.Selected >= len(field.Options) {
						field.Selected = 0
					}
					field.Value = field.Options[field.Selected].Value
					return nil
				}
				if field.Type == FieldToggle {
					field.BoolValue = !field.BoolValue
					return nil
				}
			}

		case " ":
			if f.FocusIndex < len(f.Fields) {
				field := &f.Fields[f.FocusIndex]
				if field.Type == FieldToggle {
					field.BoolValue = !field.BoolValue
					return nil
				}
			}
		}
	}

	// Update the focused field
	if f.FocusIndex < len(f.Fields) {
		field := &f.Fields[f.FocusIndex]
		switch field.Type {
		case FieldInput:
			var cmd tea.Cmd
			field.textInput, cmd = field.textInput.Update(msg)
			field.Value = field.textInput.Value()
			cmds = append(cmds, cmd)

		case FieldPath:
			prevValue := field.textInput.Value()
			var cmd tea.Cmd
			field.textInput, cmd = field.textInput.Update(msg)
			field.Value = field.textInput.Value()
			cmds = append(cmds, cmd)
			// Update suggestions if value changed
			if field.Value != prevValue {
				suggestions := computePathSuggestions(field.Value, field.DirsOnly, field.AllowedExts)
				field.textInput.SetSuggestions(suggestions)
			}

		case FieldTextArea:
			var cmd tea.Cmd
			field.textArea, cmd = field.textArea.Update(msg)
			field.Value = field.textArea.Value()
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// renderFieldBlock renders a single field to a string (no trailing newline)
func (f *Form) renderFieldBlock(idx int) string {
	field := f.Fields[idx]
	isFocused := idx == f.FocusIndex
	var b strings.Builder

	// Label
	if isFocused {
		b.WriteString(focusedStyle.Render("▸ " + field.Label))
	} else {
		b.WriteString(labelStyle.Render("  " + field.Label))
	}

	// Description
	if field.Description != "" {
		b.WriteString("\n  " + descStyle.Render(field.Description))
	}

	// Field content
	switch field.Type {
	case FieldInput, FieldPath:
		b.WriteString("\n  " + field.textInput.View())

	case FieldTextArea:
		taView := field.textArea.View()
		for _, line := range strings.Split(taView, "\n") {
			b.WriteString("\n  " + line)
		}

	case FieldSelect:
		const maxVisible = 5
		totalOpts := len(field.Options)

		startIdx := 0
		endIdx := totalOpts
		if totalOpts > maxVisible {
			startIdx = field.Selected - maxVisible/2
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + maxVisible
			if endIdx > totalOpts {
				endIdx = totalOpts
				startIdx = endIdx - maxVisible
			}
		}

		var opts []string
		if startIdx > 0 {
			opts = append(opts, descStyle.Render("◀"))
		}
		for j := startIdx; j < endIdx; j++ {
			opt := field.Options[j]
			if j == field.Selected {
				style := lipgloss.NewStyle().
					Background(DraculaPurple).
					Foreground(DraculaBackground).
					Bold(true).
					Padding(0, 1)
				opts = append(opts, style.Render(opt.Label))
			} else {
				style := lipgloss.NewStyle().
					Background(colorSelection).
					Foreground(DraculaForeground).
					Padding(0, 1)
				opts = append(opts, style.Render(opt.Label))
			}
		}
		if endIdx < totalOpts {
			opts = append(opts, descStyle.Render("▶"))
		}
		b.WriteString("\n  " + strings.Join(opts, " "))

	case FieldToggle:
		selectedBox := lipgloss.NewStyle().
			Background(DraculaPurple).
			Foreground(DraculaBackground).
			Bold(true).
			Padding(0, 1)
		unselectedBox := lipgloss.NewStyle().
			Background(colorSelection).
			Foreground(DraculaForeground).
			Padding(0, 1)
		var toggle string
		if field.BoolValue {
			toggle = selectedBox.Render("ON") + " " + unselectedBox.Render("OFF")
		} else {
			toggle = unselectedBox.Render("ON") + " " + selectedBox.Render("OFF")
		}
		b.WriteString("\n  " + toggle)
	}

	return b.String()
}

// View renders the form
func (f *Form) View() string {
	var b strings.Builder

	// Collect visible field indices
	visible := make([]int, 0, len(f.Fields))
	for i, field := range f.Fields {
		if !field.Hidden {
			visible = append(visible, i)
		}
	}

	colWidth := f.Width / 2
	colStyle := lipgloss.NewStyle().Width(colWidth)

	vi := 0
	for vi < len(visible) {
		idx := visible[vi]

		// Check if next visible field wants to be inline with this one
		if vi+1 < len(visible) && f.Fields[visible[vi+1]].InlineWithPrev {
			nextIdx := visible[vi+1]
			left := colStyle.Render(f.renderFieldBlock(idx))
			right := colStyle.Render(f.renderFieldBlock(nextIdx))
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
			b.WriteString("\n\n")
			vi += 2
		} else {
			b.WriteString(f.renderFieldBlock(idx))
			b.WriteString("\n\n")
			vi++
		}
	}

	// Help text
	b.WriteString(helpStyle.Render("↑/↓/tab: navigate • ←/→: select • ctrl+n: newline • ctrl+s: submit"))

	return b.String()
}

// Submitted returns true if the form was submitted
func (f *Form) Submitted() bool {
	return f.submitted
}

// GetString returns a string value by key
func (f *Form) GetString(key string) string {
	for _, field := range f.Fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}

// GetBool returns a bool value by key
func (f *Form) GetBool(key string) bool {
	for _, field := range f.Fields {
		if field.Key == key {
			return field.BoolValue
		}
	}
	return false
}

// SetError sets an error message
func (f *Form) SetError(msg string) {
	f.errorMsg = msg
	f.submitted = false
}

// Reset resets the submitted state
func (f *Form) Reset() {
	f.submitted = false
}

// SetFieldHidden sets the hidden state of a field by key
func (f *Form) SetFieldHidden(key string, hidden bool) {
	for i := range f.Fields {
		if f.Fields[i].Key == key {
			f.Fields[i].Hidden = hidden
			// If the currently focused field just became hidden, move to next visible
			if hidden && f.FocusIndex == i {
				f.FocusIndex = f.nextVisibleField(i)
				f.focusField(f.FocusIndex)
			}
			return
		}
	}
}

// nextVisibleField returns the next visible field index, wrapping around
func (f *Form) nextVisibleField(from int) int {
	n := len(f.Fields)
	for i := 1; i <= n; i++ {
		idx := (from + i) % n
		if !f.Fields[idx].Hidden {
			return idx
		}
	}
	return from // fallback: all hidden (shouldn't happen)
}

// prevVisibleField returns the previous visible field index, wrapping around
func (f *Form) prevVisibleField(from int) int {
	n := len(f.Fields)
	for i := 1; i <= n; i++ {
		idx := (from - i + n) % n
		if !f.Fields[idx].Hidden {
			return idx
		}
	}
	return from
}

// computePathSuggestions generates path suggestions
func computePathSuggestions(input string, dirsOnly bool, allowedExts []string) []string {
	var dir, prefix string

	// Handle tilde expansion for reading directory
	expandedInput := api.ExpandTilde(input)
	hasTilde := strings.HasPrefix(input, "~")

	if input == "" {
		dir = "."
		prefix = ""
	} else if strings.HasSuffix(expandedInput, "/") {
		dir = expandedInput
		prefix = ""
	} else {
		dir = filepath.Dir(expandedInput)
		prefix = filepath.Base(expandedInput)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Must match prefix (case-insensitive)
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		// Build full suggestion path
		var suggestion string
		if entry.IsDir() {
			if dir == "." {
				suggestion = name + "/"
			} else if strings.HasSuffix(expandedInput, "/") {
				suggestion = expandedInput + name + "/"
			} else {
				suggestion = filepath.Dir(expandedInput) + "/" + name + "/"
			}
		} else {
			// Skip files if dirsOnly
			if dirsOnly {
				continue
			}
			// Check extension filter
			if len(allowedExts) > 0 {
				ext := strings.ToLower(filepath.Ext(name))
				if !slices.Contains(allowedExts, ext) {
					continue
				}
			}
			if dir == "." {
				suggestion = name
			} else if strings.HasSuffix(expandedInput, "/") {
				suggestion = expandedInput + name
			} else {
				suggestion = filepath.Dir(expandedInput) + "/" + name
			}
		}

		// Convert back to tilde format if input used tilde
		if hasTilde {
			suggestion = "~" + strings.TrimPrefix(suggestion, api.ExpandTilde("~"))
		}

		// Only include suggestions that extend the current input
		if len(suggestion) > len(input) && strings.HasPrefix(suggestion, input) {
			results = append(results, suggestion)
		}
	}

	return results
}

// AspectRatioOptions returns standard aspect ratio options
// Auto (empty string) means aspectRatio is not sent in the request
func AspectRatioOptions() []SelectOption {
	return []SelectOption{
		{Label: "Auto", Value: ""},
		{Label: "1:1", Value: "1:1"},
		{Label: "16:9", Value: "16:9"},
		{Label: "9:16", Value: "9:16"},
		{Label: "4:3", Value: "4:3"},
		{Label: "3:4", Value: "3:4"},
		{Label: "2:3", Value: "2:3"},
		{Label: "3:2", Value: "3:2"},
		{Label: "5:4", Value: "5:4"},
		{Label: "4:5", Value: "4:5"},
		{Label: "21:9", Value: "21:9"},
	}
}

// ImageSizeOptions returns standard image size options
func ImageSizeOptions() []SelectOption {
	return []SelectOption{
		{Label: "1K", Value: "1K"},
		{Label: "2K", Value: "2K"},
		{Label: "4K", Value: "4K"},
	}
}

// ModelOptions returns available model options
func ModelOptions() []SelectOption {
	return []SelectOption{
		{Label: "Pro", Value: "pro"},
		{Label: "Flash", Value: "flash"},
	}
}

// ThinkingLevelOptions returns available thinking level options (Flash only)
func ThinkingLevelOptions() []SelectOption {
	return []SelectOption{
		{Label: "Minimal", Value: "minimal"},
		{Label: "High", Value: "high"},
	}
}

// FormStyles for external use
type FormStyles struct {
	Window lipgloss.Style
	Title  lipgloss.Style
	Help   lipgloss.Style
}

// DefaultFormStyles returns default form styles
func DefaultFormStyles() FormStyles {
	return FormStyles{
		Window: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DraculaPurple).
			Padding(1, 2),
		Title: titleStyle,
		Help:  helpStyle,
	}
}

// BackToMenuMsg signals returning to the main menu
type BackToMenuMsg struct{}
