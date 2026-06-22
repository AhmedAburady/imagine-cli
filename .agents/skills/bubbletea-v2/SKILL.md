---
name: bubbletea-v2
description: >-
  Generates, explains, and debugs terminal UI applications using Charm's
  BubbleTea v2 framework for Go. Use when building TUI apps with the Elm
  Architecture, handling keyboard/mouse input, managing async commands,
  implementing progress bars, alternate screens, clipboard, or composing
  multi-view terminal programs. Covers Model/Update/View pattern, message
  routing, command batching, color detection, and external process execution.
argument-hint: [component|pattern|feature description]
allowed-tools: Read, Grep, Bash(go doc *)
metadata:
  author: skill-generator
  version: "1.0"
  category: go-cli
---

# BubbleTea v2 — Terminal UI Framework for Go

BubbleTea v2 (`charm.land/bubbletea/v2`) implements **The Elm Architecture** for terminal UIs.
Every program has three responsibilities: hold state (Model), react to events (Update), and render (View).

## Quick Reference

- **API Reference** → [references/API-REFERENCE.md](references/API-REFERENCE.md)
- **Key Handling** → [references/KEYS.md](references/KEYS.md)
- **Terminal Features** → [references/TERMINAL-FEATURES.md](references/TERMINAL-FEATURES.md)
- **View Features** → [references/VIEW-FEATURES.md](references/VIEW-FEATURES.md)
- **Common Patterns** → [references/PATTERNS.md](references/PATTERNS.md)
- **Architecture** → [references/ARCHITECTURE.md](references/ARCHITECTURE.md)
- **Runnable Examples** → [references/_examples/](references/_examples/) (60+ programs)
- **Migrating from v1?** → [references/UPGRADE_GUIDE_V2.md](references/UPGRADE_GUIDE_V2.md)

---

## Migrating from BubbleTea v1?

If the codebase uses `github.com/charmbracelet/bubbletea` (v1), read the full
migration guide before making any changes:

**[references/UPGRADE_GUIDE_V2.md](references/UPGRADE_GUIDE_V2.md)**

Key breaking changes at a glance:

| Area | v1 | v2 |
|---|---|---|
| Import | `github.com/charmbracelet/bubbletea` | `charm.land/bubbletea/v2` |
| `View()` return | `string` | `tea.View` (use `tea.NewView(s)`) |
| Key press | `case tea.KeyMsg:` | `case tea.KeyPressMsg:` |
| Space bar | `case " ":` | `case "space":` |
| Mouse data | `msg.X, msg.Y` | `msg.Mouse().X, msg.Mouse().Y` |
| Mouse button | `tea.MouseButtonLeft` | `tea.MouseLeft` |
| Alt screen | `tea.WithAltScreen()` option | `view.AltScreen = true` in `View()` |
| Mouse mode | `tea.WithMouseCellMotion()` option | `view.MouseMode = tea.MouseModeCellMotion` in `View()` |
| Program start | `p.Start()` | `p.Run()` |

---

## Core Interface

```go
type Model interface {
    Init()           tea.Cmd              // startup command (or nil)
    Update(tea.Msg) (tea.Model, tea.Cmd) // pure state transition
    View()           tea.View            // pure rendering
}
```

### Minimal working program

```go
package main

import (
    "fmt"
    "os"
    tea "charm.land/bubbletea/v2"
)

type model struct{ count int }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up":
            m.count++
        case "down":
            m.count--
        }
    }
    return m, nil
}

func (m model) View() tea.View {
    return tea.NewView(fmt.Sprintf("Count: %d\n\n(↑/↓ to change, q to quit)", m.count))
}

func main() {
    if _, err := tea.NewProgram(model{}).Run(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```

---

## Key Handling

```go
case tea.KeyPressMsg:
    // String-based (convenient)
    switch msg.String() {
    case "enter", "ctrl+c", "q", "alt+left":
    }

    // Code-based (type-safe)
    switch msg.Key().Code {
    case tea.KeyEnter:
    case tea.KeyEscape:
    case tea.KeyF1:
    default:
        // printable character
        text := msg.Key().Text
    }

    // Modifiers
    key := msg.Key()
    if key.Mod&tea.ModCtrl != 0 { /* ctrl held */ }
    if key.IsRepeat { /* key held down */ }
```

**See [references/KEYS.md](references/KEYS.md) for complete key constants and patterns.**

---

## Mouse Handling

Enable in View, then handle messages:

```go
// Enable in View()
func (m model) View() tea.View {
    v := tea.NewView(content)
    v.MouseMode = tea.MouseModeCellMotion  // clicks + wheels
    return v
}

// Handle in Update()
case tea.MouseClickMsg:
    x, y := msg.X, msg.Y
    if msg.Button == tea.MouseLeft { /* ... */ }
case tea.MouseWheelMsg:
    if msg.Button == tea.MouseWheelUp { /* scroll up */ }
```

---

## Async Commands

```go
// Define a command (runs in goroutine, returns a Msg)
func fetchData(url string) tea.Cmd {
    return func() tea.Msg {
        resp, err := http.Get(url)
        return dataMsg{resp, err}
    }
}

// Batch: run concurrently (no order guarantee)
return m, tea.Batch(fetchData(url), spinnerTickCmd())

// Sequence: run in order
return m, tea.Sequence(initCmd, startCmd)

// Recurring ticks
func tick() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

---

## View Features

```go
func (m model) View() tea.View {
    v := tea.NewView(content)

    // Full-screen alternate buffer
    v.AltScreen = true

    // Mouse support
    v.MouseMode = tea.MouseModeCellMotion   // or MouseModeAllMotion

    // Terminal title
    v.WindowTitle = "My App"

    // Cursor control
    v.Cursor = &tea.Cursor{
        Shape: tea.CursorBar,  // Block, Underline, Bar
        Blink: true,
    }

    // Focus events (FocusMsg / BlurMsg)
    v.ReportFocus = true

    // Keyboard enhancements (key release detection)
    v.KeyboardEnhancements.ReportEventTypes = true

    return v
}
```

**See [references/VIEW-FEATURES.md](references/VIEW-FEATURES.md) for complete View documentation.**

---

## Terminal Color Detection

```go
func (m model) Init() tea.Cmd {
    return tea.RequestBackgroundColor()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        if msg.IsDark() {
            m.styles = darkTheme()
        } else {
            m.styles = lightTheme()
        }
    }
    return m, nil
}
```

**See [references/TERMINAL-FEATURES.md](references/TERMINAL-FEATURES.md) for color, focus, and keyboard enhancement details.**

---

## Program Options

```go
tea.NewProgram(model,
    tea.WithContext(ctx),           // cancellation
    tea.WithOutput(w),              // custom writer
    tea.WithFPS(30),                // 1–120, default 60
    tea.WithoutRenderer(),          // headless/testing
    tea.WithWindowSize(80, 24),     // testing override
    tea.WithFilter(filterFn),       // intercept messages
)
```

---

## Debugging

```go
// Log to file (stdout is occupied by TUI)
f, _ := tea.LogToFile("debug.log", "debug")
defer f.Close()

// Or via env vars
TEA_TRACE=trace.log go run .
TEA_DEBUG=true go run .
```

---

## External Process Execution

```go
// Pause TUI, open editor, resume
case openEditorMsg:
    cmd := exec.Command("vim", filename)
    return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
        return editorClosedMsg{err}
    })
```

---

## Clipboard (OSC52)

```go
// Write
return m, tea.SetClipboard("text to copy")

// Read — triggers ClipboardMsg
return m, tea.ReadClipboard()

case tea.ClipboardMsg:
    m.clipboardContent = string(msg)
```

---

## Error Types

| Error | Cause |
|---|---|
| `tea.ErrInterrupted` | Ctrl+C or `InterruptMsg` |
| `tea.ErrProgramKilled` | External SIGKILL |
| `tea.ErrProgramPanic` | Recovered panic |

---

## Examples Index

Full source for each example lives in `references/_examples/<name>/main.go`.
Read them to understand real usage patterns.

| Example | Demonstrates |
|---|---|
| `simple` | Minimal program skeleton |
| `views` / `composable-views` | Multi-view layout and routing |
| `textinput` / `textinputs` | Single and multi-field text input |
| `textarea` | Multi-line text editing |
| `list-simple` / `list-default` / `list-fancy` | List navigation, filtering, custom delegates |
| `table` / `table-resize` | Tabular data display |
| `tabs` | Tabbed navigation |
| `spinner` / `spinners` | Loading spinners |
| `progress-bar` / `progress-animated` / `progress-download` | Progress bars |
| `stopwatch` / `timer` | Time-based UI |
| `paginator` | Page navigation |
| `pager` | Scrollable long content |
| `mouse` / `clickable` | Mouse click and hover handling |
| `keyboard-enhancements` / `print-key` | Key events and modifier detection |
| `focus-blur` | Terminal focus/blur events |
| `chat` | Streaming messages, viewport scroll |
| `http` | Async HTTP with Cmd |
| `realtime` | Real-time data updates |
| `exec` | Pause TUI, run external command, resume |
| `file-picker` | Filesystem navigation |
| `altscreen-toggle` | Switching in/out of alternate screen |
| `fullscreen` | Full-window TUI |
| `colorprofile` / `set-terminal-color` | Color detection and terminal theming |
| `cursor-style` | Cursor shape and blink |
| `set-window-title` | Terminal window title |
| `clipboard` (capability) | OSC52 clipboard read/write |
| `send-msg` | Sending messages from outside the program |
| `sequence` / `debounce` | Command ordering and input debounce |
| `split-editors` | Side-by-side editor panes |
| `package-manager` | Column layout with status |
| `isbn-form` / `query-term` / `autocomplete` | Form patterns |
| `pipe` | Reading stdin in non-TTY mode |
| `tui-daemon-combo` | TUI + background daemon |
| `suspend` | Ctrl+Z suspend/resume |
| `prevent-quit` | Intercepting quit for confirmation |
| `help` | Key binding help overlay |
| `result` | Returning a value from a TUI program |
| `doom-fire` / `eyes` / `canvas` / `splash` | Animation and graphics |
| `glamour` | Markdown rendering in TUI |

---

## Workflow for Building a TUI App

1. **Define your Model struct** — all state lives here
2. **Implement Init()** — return startup commands (data fetch, color query, etc.)
3. **Implement Update()** — switch on `tea.Msg` types, return new model + next command
4. **Implement View()** — build a string with lipgloss styling, return `tea.NewView(s)`
5. **Wire up `tea.NewProgram`** and call `.Run()`
6. **Add components** (bubbles library) for spinners, lists, inputs, viewports

For complex layouts, compose multiple sub-models each with their own `Update` and `View`, routing messages from a parent model. See [references/PATTERNS.md](references/PATTERNS.md) for composable views.
