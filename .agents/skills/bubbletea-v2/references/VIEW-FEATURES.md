# BubbleTea v2 — View Features Reference

## View Struct

```go
type View struct {
    Content                  Layer              // String, fmt.Stringer, or Layer
    Cursor                   *Cursor            // Cursor position and style
    BackgroundColor          color.Color        // Terminal background color
    ForegroundColor          color.Color        // Terminal foreground color  
    WindowTitle              string             // Terminal window title
    ProgressBar              *ProgressBar       // Native progress bar
    AltScreen                bool               // Use alternate screen buffer
    ReportFocus              bool               // Enable FocusMsg/BlurMsg
    DisableBracketedPasteMode bool              // Disable bracketed paste
    MouseMode                MouseMode          // Mouse event mode
    KeyboardEnhancements     KeyboardEnhancements // Enhanced keyboard features
}
```

---

## Creating Views

### Simple String Content

```go
func (m model) View() tea.View {
    return tea.NewView("Hello, World!")
}
```

### Styled Content

```go
func (m model) View() tea.View {
    styled := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("15")).
        Render("Bold Text")
    return tea.NewView(styled)
}
```

### Dynamic Content

```go
func (m model) View() tea.View {
    content := fmt.Sprintf("Count: %d\n\nPress q to quit.", m.count)
    return tea.NewView(content)
}
```

### Using SetContent

```go
func (m model) View() tea.View {
    var v tea.View
    v.SetContent("Hello, World!")
    v.AltScreen = true
    return v
}
```

### Composable Layers

```go
func (m model) View() tea.View {
    sidebar := lipgloss.NewLayer(m.sidebarView()).
        X(0).Y(0).
        Width(20).Height(m.height)
    
    content := lipgloss.NewLayer(m.contentView()).
        X(22).Y(0).
        Width(m.width - 22).Height(m.height)
    
    canvas := lipgloss.NewCanvas(sidebar, content)
    return tea.NewView(canvas)
}
```

---

## Alt Screen

The alternate screen buffer provides a full-screen experience that doesn't affect terminal scrollback.

```go
func (m model) View() tea.View {
    v := tea.NewView("Full screen application")
    v.AltScreen = true
    return v
}
```

When `AltScreen = true`:
- Terminal enters alternate buffer mode
- Original terminal content is preserved
- On exit, terminal returns to original buffer
- `tea.Printf` and `tea.Println` are suppressed

---

## Mouse Modes

### MouseModeNone (Default)

No mouse events are generated.

```go
v.MouseMode = tea.MouseModeNone
```

### MouseModeCellMotion (Recommended)

Generates events for:
- Mouse clicks
- Mouse releases
- Mouse wheel scrolls
- Mouse motion while a button is held (drag)

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.MouseMode = tea.MouseModeCellMotion
    return v
}
```

### MouseModeAllMotion

Generates events for all mouse movement, even without buttons pressed. High traffic — use for drawing apps or games.

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.MouseMode = tea.MouseModeAllMotion
    return v
}
```

---

## Cursor Control

### Cursor Struct

```go
type Cursor struct {
    Position Position     // X, Y coordinates
    Color    color.Color  // Cursor color
    Shape    CursorShape  // Block, Underline, or Bar
    Blink    bool         // Enable blinking
}

type Position struct {
    X, Y int
}
```

### Cursor Shapes

| Constant | Description |
|---|---|
| `CursorBlock` | Solid block cursor |
| `CursorUnderline` | Underline cursor |
| `CursorBar` | Vertical I-beam cursor |

### Setting Cursor

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.Cursor = &tea.Cursor{
        Position: tea.Position{X: 5, Y: 3},
        Shape:    tea.CursorBar,
        Blink:    true,
        Color:    lipgloss.Color("#00ff00"),
    }
    return v
}
```

### Using NewCursor Helper

```go
v.Cursor = tea.NewCursor(5, 3)
v.Cursor.Shape = tea.CursorBlock
v.Cursor.Blink = true
```

### Hiding Cursor

```go
v.Cursor = nil
```

### Text Input Cursor

For text inputs, use the cursor from bubbles/textinput or bubbles/textarea:

```go
func (m model) View() tea.View {
    v := tea.NewView(m.textinput.View())
    v.Cursor = m.textinput.Cursor()
    return v
}
```

---

## Terminal Colors

### Setting Background Color

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.BackgroundColor = lipgloss.Color("#1a1a2e")
    return v
}
```

### Setting Foreground Color

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.ForegroundColor = lipgloss.Color("#ffffff")
    return v
}
```

### Resetting to Default

```go
v.BackgroundColor = nil  // Reset to terminal default
v.ForegroundColor = nil  // Reset to terminal default
```

---

## Window Title

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.WindowTitle = "My Application"
    return v
}
```

### Dynamic Title

```go
func (m model) View() tea.View {
    title := m.appName
    if m.modified {
        title = "* " + title
    }
    if m.filename != "" {
        title += " - " + m.filename
    }
    
    v := tea.NewView("...")
    v.WindowTitle = title
    return v
}
```

---

## Focus Reporting

Enable to receive `tea.FocusMsg` and `tea.BlurMsg` when the terminal gains or loses focus.

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.ReportFocus = true
    return v
}
```

---

## Bracketed Paste

Bracketed paste mode is enabled by default. When disabled, pasted text appears as individual key events.

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.DisableBracketedPasteMode = true
    return v
}
```

With bracketed paste enabled (default), you receive:
- `tea.PasteStartMsg` - Paste begins
- `tea.PasteMsg{Content: "pasted text"}` - Paste content
- `tea.PasteEndMsg` - Paste ends

---

## Keyboard Enhancements

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.KeyboardEnhancements = tea.KeyboardEnhancements{
        ReportEventTypes: true,
    }
    return v
}
```

---

## Progress Bar

Native terminal progress bar (Windows Terminal, iTerm2, etc.)

```go
func (m model) View() tea.View {
    v := tea.NewView("Processing...")
    v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, m.progress)
    return v
}
```

### States

| Constant | Description |
|---|---|
| `ProgressBarNone` | Hidden |
| `ProgressBarDefault` | Normal (blue) |
| `ProgressBarError` | Error (red) |
| `ProgressBarIndeterminate` | Spinner |
| `ProgressBarWarning` | Warning (yellow) |

---

## Complete View Example

```go
func (m model) View() tea.View {
    content := renderContent(m)
    
    v := tea.NewView(content)
    
    // Full screen mode
    v.AltScreen = true
    
    // Mouse support for lists/tables
    v.MouseMode = tea.MouseModeCellMotion
    
    // Window title
    v.WindowTitle = fmt.Sprintf("%s - %d items", m.appName, len(m.items))
    
    // Cursor for text input
    if m.textinput.Focused() {
        v.Cursor = m.textinput.Cursor()
    } else {
        v.Cursor = nil
    }
    
    // Theme colors
    if m.isDark {
        v.BackgroundColor = lipgloss.Color("#1a1a2e")
        v.ForegroundColor = lipgloss.Color("#ffffff")
    }
    
    // Focus events
    v.ReportFocus = true
    
    // Key release detection
    v.KeyboardEnhancements.ReportEventTypes = true
    
    // Native progress bar
    if m.loading {
        v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
    } else if m.progress > 0 {
        v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, m.progress)
    }
    
    return v
}
```

---

## Layer Interface

For advanced composable views:

```go
type Layer interface {
    Draw(s Screen, r Rectangle)
}

type Hittable interface {
    Hit(x, y int) string
}
```

### Using with lipgloss Layers

```go
layer1 := lipgloss.NewLayer("Sidebar").
    X(0).Y(0).
    Width(20).Height(24)

layer2 := lipgloss.NewLayer("Main content").
    X(22).Y(0).
    Width(60).Height(24)

canvas := lipgloss.NewCanvas(layer1, layer2)
v := tea.NewView(canvas)
```
