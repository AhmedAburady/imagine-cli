# BubbleTea v2 — Terminal Features

## Color Detection

### Requesting Colors

```go
func (m model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestBackgroundColor,
        tea.RequestForegroundColor,
        tea.RequestCursorColor,
    )
}
```

### Handling Color Messages

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        m.isDark = msg.IsDark()
        m.styles = getStylesForTheme(msg.IsDark())
        m.bgColor = msg.Color()
        
    case tea.ForegroundColorMsg:
        m.fgColor = msg.Color()
        
    case tea.CursorColorMsg:
        m.cursorColor = msg.Color()
        
    case tea.ColorProfileMsg:
        // msg.Profile is colorprofile.TrueColor, Ansi256, Ansi, etc.
        m.hasTrueColor = msg.Profile == colorprofile.TrueColor
    }
    return m, nil
}
```

### Color Message Types

| Type | Method | Description |
|---|---|---|
| `BackgroundColorMsg` | `IsDark() bool`, `Color() color.Color` | Terminal background color |
| `ForegroundColorMsg` | `IsDark() bool`, `Color() color.Color` | Terminal foreground color |
| `CursorColorMsg` | `IsDark() bool`, `Color() color.Color` | Terminal cursor color |
| `ColorProfileMsg` | `Profile colorprofile.Profile` | Color capability profile |

### Light/Dark Theme Switching

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        m.styles = newStyles(msg.IsDark())
    }
    return m, nil
}

func newStyles(dark bool) styles {
    lightDark := lipgloss.LightDark(dark)
    return styles{
        primary: lipgloss.NewStyle().Foreground(lightDark(
            lipgloss.Color("235"),  // dark theme
            lipgloss.Color("252"),  // light theme
        )),
    }
}
```

---

## Focus Reporting

### Enabling Focus Events

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.ReportFocus = true
    return v
}
```

### Handling Focus/Blur

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case tea.FocusMsg:
        m.focused = true
        // Terminal gained focus
        
    case tea.BlurMsg:
        m.focused = false
        // Terminal lost focus
    }
    return m, nil
}
```

### Toggle Focus Reporting

```go
case tea.KeyPressMsg:
    if msg.String() == "f" {
        m.reportFocus = !m.reportFocus
    }

func (m model) View() tea.View {
    v := tea.NewView("...")
    v.ReportFocus = m.reportFocus
    return v
}
```

**Note:** Tmux requires configuration to report focus events. Most modern terminals support this.

---

## Keyboard Enhancements

### Requesting Enhanced Features

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.KeyboardEnhancements = tea.KeyboardEnhancements{
        ReportEventTypes: true, // Enable key release + repeat detection
    }
    return v
}
```

### KeyboardEnhancements Struct

```go
type KeyboardEnhancements struct {
    ReportEventTypes bool // Request key release and repeat events
}
```

### Detecting Terminal Support

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyboardEnhancementsMsg:
        // Basic disambiguation is always available if this message is received
        m.canDistinguishKeys = msg.SupportsKeyDisambiguation()
        
        // Can we detect key releases?
        m.canDetectRelease = msg.SupportsEventTypes()
    }
    return m, nil
}
```

### KeyboardEnhancementsMsg Methods

| Method | Description |
|---|---|
| `SupportsKeyDisambiguation() bool` | Can distinguish `enter` vs `shift+enter`, `tab` vs `ctrl+i` |
| `SupportsEventTypes() bool` | Can report key press, release, and repeat |

### Key Release Handling

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        m.keyHeld = true
        fmt.Println("Key pressed:", msg.String())
        
    case tea.KeyReleaseMsg:
        m.keyHeld = false
        fmt.Println("Key released:", msg.String())
    }
    return m, nil
}
```

---

## Window Title

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.WindowTitle = "My Application - " + m.currentFile
    return v
}
```

### Dynamic Title

```go
func (m model) View() tea.View {
    title := fmt.Sprintf("%s - %d items", m.appName, len(m.items))
    if m.modified {
        title = "* " + title
    }
    
    v := tea.NewView("...")
    v.WindowTitle = title
    return v
}
```

---

## Native Progress Bar

Windows Terminal, iTerm2, and other modern terminals support native progress bars in the tab/window decoration.

### Progress Bar States

| Constant | Description |
|---|---|
| `ProgressBarNone` | No progress bar displayed |
| `ProgressBarDefault` | Normal progress (blue) |
| `ProgressBarError` | Error state (red) |
| `ProgressBarIndeterminate` | Unknown progress (spinner) |
| `ProgressBarWarning` | Warning state (yellow) |

### Creating a Progress Bar

```go
func (m model) View() tea.View {
    v := tea.NewView("Processing...")
    v.ProgressBar = &tea.ProgressBar{
        State: tea.ProgressBarDefault,
        Value: m.progress, // 0-100
    }
    return v
}
```

### Helper Function

```go
v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, m.progress)
```

### Progress Bar Patterns

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    
    switch {
    case m.error != nil:
        v.ProgressBar = tea.NewProgressBar(tea.ProgressBarError, 0)
    case m.loading:
        v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
    default:
        v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, m.progress)
    }
    
    return v
}
```

---

## Terminal Queries

### Request Terminal Version

```go
func (m model) Init() tea.Cmd {
    return tea.RequestTerminalVersion
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.TerminalVersionMsg:
        m.terminalName = msg.Name
        // e.g., "iTerm2", "Windows Terminal", "Alacritty"
    }
    return m, nil
}
```

### Request Cursor Position

```go
func (m model) Init() tea.Cmd {
    return tea.RequestCursorPosition
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.CursorPositionMsg:
        m.cursorX = msg.X
        m.cursorY = msg.Y
    }
    return m, nil
}
```

### Request Window Size

Usually automatic via `WindowSizeMsg` on startup and resize, but can be requested:

```go
func (m model) Init() tea.Cmd {
    return tea.RequestWindowSize
}
```

### Request Terminal Capabilities

```go
func (m model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestCapability("RGB"),  // True color support
        tea.RequestCapability("Tc"),   // True color (alternate)
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.CapabilityMsg:
        // Terminal responded with capability info
        fmt.Println("Capability:", msg.Content)
    case tea.ColorProfileMsg:
        // Capability responses may upgrade the color profile
    }
    return m, nil
}
```

---

## Environment Variables (SSH Sessions)

When running in a remote session (SSH), `os.Getenv` returns server environment variables. Use `EnvMsg` to get client environment:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.EnvMsg:
        term := msg.Getenv("TERM")
        shell := msg.Getenv("SHELL")
        home := msg.Getenv("HOME")
        
        // Check if variable exists
        path, exists := msg.LookupEnv("PATH")
    }
    return m, nil
}
```

### EnvMsg Methods

| Method | Description |
|---|---|
| `Getenv(key string) string` | Get environment variable (empty if not set) |
| `LookupEnv(key string) (string, bool)` | Get with existence check |

---

## Terminal Mode Reports

Query terminal mode settings:

```go
func (m model) Init() tea.Cmd {
    return tea.Raw(ansi.RequestModeFocusEvent)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.ModeReportMsg:
        if msg.Mode == ansi.ModeFocusEvent && !msg.Value.IsNotRecognized() {
            m.supportsFocus = true
        }
    }
    return m, nil
}
```

---

## Raw ANSI Sequences

For advanced terminal control:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case input.PrimaryDeviceAttributesEvent:
        for _, attr := range msg {
            if attr == 4 {
                // Terminal supports Sixel graphics
            }
        }
    }
    
    // Send raw ANSI sequence
    return m, tea.Raw(ansi.RequestPrimaryDeviceAttributes)
}
```

---

## Complete Feature Detection Example

```go
type model struct {
    terminalName        string
    isDark              bool
    hasTrueColor        bool
    hasEnhancedKeyboard bool
    hasEventTypes       bool
    hasFocusReporting   bool
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestBackgroundColor,
        tea.RequestTerminalVersion,
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        m.isDark = msg.IsDark()
        
    case tea.TerminalVersionMsg:
        m.terminalName = msg.Name
        
    case tea.ColorProfileMsg:
        m.hasTrueColor = msg.Profile == colorprofile.TrueColor
        
    case tea.KeyboardEnhancementsMsg:
        m.hasEnhancedKeyboard = msg.SupportsKeyDisambiguation()
        m.hasEventTypes = msg.SupportsEventTypes()
    }
    return m, nil
}

func (m model) View() tea.View {
    var b strings.Builder
    fmt.Fprintf(&b, "Terminal: %s\n", m.terminalName)
    fmt.Fprintf(&b, "Dark mode: %v\n", m.isDark)
    fmt.Fprintf(&b, "True color: %v\n", m.hasTrueColor)
    fmt.Fprintf(&b, "Enhanced keyboard: %v\n", m.hasEnhancedKeyboard)
    fmt.Fprintf(&b, "Event types: %v\n", m.hasEventTypes)
    
    v := tea.NewView(b.String())
    v.KeyboardEnhancements.ReportEventTypes = true
    v.ReportFocus = true
    return v
}
```
