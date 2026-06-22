# BubbleTea v2 — Common Patterns

## Composable Views (Multi-Model)

Break a complex app into focused sub-models. Each manages its own state.

```go
type parentModel struct {
    sidebar sidebarModel
    content contentModel
    focus   int  // 0=sidebar, 1=content
}

func (m parentModel) Init() tea.Cmd {
    return tea.Batch(m.sidebar.Init(), m.content.Init())
}

func (m parentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    var cmd tea.Cmd

    // Route messages to focused child
    switch m.focus {
    case 0:
        m.sidebar, cmd = m.sidebar.Update(msg)
    case 1:
        m.content, cmd = m.content.Update(msg)
    }
    cmds = append(cmds, cmd)

    // Some messages go to both children
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.sidebar, cmd = m.sidebar.Update(msg)
        cmds = append(cmds, cmd)
        m.content, cmd = m.content.Update(msg)
        cmds = append(cmds, cmd)
    case tea.KeyPressMsg:
        if msg.String() == "tab" {
            m.focus = (m.focus + 1) % 2
        }
    }

    return m, tea.Batch(cmds...)
}

func (m parentModel) View() tea.View {
    return tea.NewView(
        lipgloss.JoinHorizontal(lipgloss.Top,
            m.sidebar.View().Content,
            m.content.View().Content,
        ),
    )
}
```

---

## Spinner / Loading State

```go
import "charm.land/bubbles/v2/spinner"

type model struct {
    spinner  spinner.Model
    loading  bool
    result   string
}

func initialModel() model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    return model{spinner: s, loading: true}
}

func (m model) Init() tea.Cmd {
    return tea.Batch(m.spinner.Tick, fetchData())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case spinner.TickMsg:
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    case dataReadyMsg:
        m.loading = false
        m.result = msg.data
        return m, nil
    }
    return m, nil
}

func (m model) View() tea.View {
    if m.loading {
        return tea.NewView(m.spinner.View() + " Loading...")
    }
    return tea.NewView(m.result)
}
```

---

## Pagination / List Navigation

```go
type model struct {
    items    []string
    cursor   int
    offset   int
    height   int   // terminal height (from WindowSizeMsg)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.height = msg.Height - 2  // leave room for header/footer
    case tea.KeyPressMsg:
        switch msg.String() {
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
                if m.cursor < m.offset {
                    m.offset--
                }
            }
        case "down", "j":
            if m.cursor < len(m.items)-1 {
                m.cursor++
                if m.cursor >= m.offset+m.height {
                    m.offset++
                }
            }
        }
    }
    return m, nil
}

func (m model) View() tea.View {
    var sb strings.Builder
    visible := m.items[m.offset:min(m.offset+m.height, len(m.items))]
    for i, item := range visible {
        idx := m.offset + i
        if idx == m.cursor {
            sb.WriteString("> " + item + "\n")
        } else {
            sb.WriteString("  " + item + "\n")
        }
    }
    return tea.NewView(sb.String())
}
```

---

## Debounce / Throttle Input

```go
type debouncedSearchMsg struct {
    query string
    id    int
}

func debounce(query string, id int) tea.Cmd {
    return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
        return debouncedSearchMsg{query, id}
    })
}

type model struct {
    input     textinput.Model
    query     string
    pendingID int  // generation counter to discard stale ticks
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        var cmd tea.Cmd
        m.input, cmd = m.input.Update(msg)
        // start new debounce, bump generation
        m.pendingID++
        return m, tea.Batch(cmd, debounce(m.input.Value(), m.pendingID))
    case debouncedSearchMsg:
        if msg.id != m.pendingID {
            return m, nil // stale, ignore
        }
        return m, search(msg.query)
    }
    return m, nil
}
```

---

## Alternate Screen (Full-Window)

```go
func (m model) View() tea.View {
    v := tea.NewView(renderFullScreen(m))
    v.AltScreen = true
    return v
}
// AltScreen is automatically restored on program quit.
```

---

## Confirmation Dialog

```go
type confirmModel struct {
    message  string
    onYes    tea.Cmd
    onNo     tea.Cmd
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "y", "Y", "enter":
            return m, m.onYes
        case "n", "N", "escape", "q":
            return m, m.onNo
        }
    }
    return m, nil
}

func (m confirmModel) View() tea.View {
    return tea.NewView(m.message + "\n\n[y] Yes  [n] No")
}
```

---

## Progress Bar

```go
type progressMsg float64

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case progressMsg:
        if float64(msg) >= 1.0 {
            return m, tea.Quit
        }
        m.progress = float64(msg)
        return m, doNextChunk(m.progress)
    }
    return m, nil
}

func (m model) View() tea.View {
    v := tea.NewView(renderProgressBar(m.progress, 40))
    v.ProgressBar = &tea.ProgressBar{
        State: tea.ProgressBarDefault,
        Value: int(m.progress * 100),
    }
    return v
}

func renderProgressBar(pct float64, width int) string {
    filled := int(pct * float64(width))
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    return fmt.Sprintf("[%s] %.0f%%", bar, pct*100)
}
```

---

## Window Resize Handling

Always handle `WindowSizeMsg` to keep layout correct:

```go
type model struct {
    width  int
    height int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }
    return m, nil
}
```

---

## Custom Message Bus (Decoupled Components)

```go
// Define domain messages
type userSelectedMsg struct{ id int }
type dataLoadedMsg   struct{ items []Item }

// Component A sends:
return m, func() tea.Msg { return userSelectedMsg{id: selected} }

// Parent routes to Component B:
case userSelectedMsg:
    return m, loadDataForUser(msg.id)

case dataLoadedMsg:
    m.list, cmd = m.list.Update(msg)
    return m, cmd
```

---

## Testing Without a Terminal

```go
func TestUpdate(t *testing.T) {
    m := initialModel()

    // Simulate messages
    next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
    assert.Nil(t, cmd)

    view := next.(model).View()
    assert.Contains(t, view.Content, "expected text")
}

// Run headless program
p := tea.NewProgram(model{},
    tea.WithoutRenderer(),
    tea.WithWindowSize(80, 24),
    tea.WithInput(strings.NewReader("q\n")),
)
finalModel, err := p.Run()
```

---

## Prevent Quit with Confirmation

```go
func filter(m tea.Model, msg tea.Msg) tea.Msg {
    if _, ok := msg.(tea.QuitMsg); !ok {
        return msg
    }
    
    model := m.(myModel)
    if model.hasChanges {
        return nil  // Block quit
    }
    return msg
}

p := tea.NewProgram(model{}, tea.WithFilter(filter))
```

---

## Focus/Blur Handling

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.ReportFocus = true
    return v
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case tea.FocusMsg:
        m.focused = true
        // Terminal gained focus
    case tea.BlurMsg:
        m.focused = false
        // Terminal lost focus - maybe pause a timer?
    }
    return m, nil
}
```

---

## Mouse Click Handling

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.MouseMode = tea.MouseModeCellMotion
    return v
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.MouseClickMsg:
        x, y := msg.X, msg.Y
        
        switch msg.Button {
        case tea.MouseLeft:
            // Handle left click at (x, y)
        case tea.MouseRight:
            // Handle right click
        }
        
    case tea.MouseWheelMsg:
        switch msg.Button {
        case tea.MouseWheelUp:
            m.scrollUp()
        case tea.MouseWheelDown:
            m.scrollDown()
        }
    }
    return m, nil
}
```

---

## Key Release Detection

Requires keyboard enhancements:

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.KeyboardEnhancements.ReportEventTypes = true
    return v
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyboardEnhancementsMsg:
        m.canDetectRelease = msg.SupportsEventTypes()
        
    case tea.KeyPressMsg:
        m.keyHeld = true
        fmt.Println("Pressed:", msg.String())
        
    case tea.KeyReleaseMsg:
        m.keyHeld = false
        fmt.Println("Released:", msg.String())
    }
    return m, nil
}
```

---

## External Process Execution

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case openEditorMsg:
        cmd := exec.Command("vim", m.filename)
        return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
            return editorClosedMsg{err}
        })
        
    case editorClosedMsg:
        if msg.err != nil {
            return m, tea.Printf("Editor error: %v", msg.err)
        }
        // Reload file contents
        return m, reloadFile(m.filename)
    }
    return m, nil
}
```

---

## Light/Dark Theme Detection

```go
func (m model) Init() tea.Cmd {
    return tea.RequestBackgroundColor
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        m.isDark = msg.IsDark()
        m.styles = newStyles(msg.IsDark())
    }
    return m, nil
}

func newStyles(dark bool) styles {
    lightDark := lipgloss.LightDark(dark)
    return styles{
        primary: lipgloss.NewStyle().Foreground(
            lightDark(lipgloss.Color("235"), lipgloss.Color("252")),
        ),
    }
}
```

---

## Clipboard Operations

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "ctrl+c":
            // Copy to clipboard
            return m, tea.SetClipboard(m.selectedText)
        case "ctrl+v":
            // Paste from clipboard
            return m, tea.ReadClipboard()
        }
        
    case tea.ClipboardMsg:
        m.buffer = string(msg.Content)
    }
    return m, nil
}
```

---

## Sending Messages from Outside

```go
func main() {
    p := tea.NewProgram(model{})
    
    // Start program in goroutine
    go func() {
        time.Sleep(5 * time.Second)
        // Send message from outside
        p.Send(externalEventMsg{data: "something happened"})
    }()
    
    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
```

---

## Context Cancellation

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Cancel after timeout
    go func() {
        time.Sleep(30 * time.Second)
        cancel()
    }()
    
    p := tea.NewProgram(model{}, tea.WithContext(ctx))
    _, err := p.Run()
    
    if errors.Is(err, tea.ErrProgramKilled) {
        fmt.Println("Program cancelled")
    }
}
```

---

## Logging in TUI Mode

Since stdout is occupied by the TUI, log to a file:

```go
func main() {
    f, err := tea.LogToFile("debug.log", "myapp: ")
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    defer f.Close()
    
    p := tea.NewProgram(model{})
    p.Run()
}
```

Or via environment:
```bash
TEA_TRACE=trace.log go run .
```
