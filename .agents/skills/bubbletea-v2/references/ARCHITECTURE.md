# BubbleTea v2 — Architecture Reference

## The Elm Architecture

BubbleTea models the [Elm Architecture](https://guide.elm-lang.org/architecture/):

```
┌─────────────────────────────────────────────────────────┐
│                      Program Loop                       │
│                                                         │
│  Terminal  ──► Msg  ──► Update(Model, Msg)             │
│                                   │                     │
│                                   ▼                     │
│  Render  ◄──  View(Model)   new Model + Cmd            │
│    │                                  │                 │
│    ▼                                  ▼                 │
│  stdout               goroutine runs Cmd ──► Msg        │
└─────────────────────────────────────────────────────────┘
```

**Key invariants:**
- `Update` and `View` are pure — no I/O, no side effects
- All I/O happens in `Cmd` goroutines
- `Model` is immutable: `Update` returns a *new* model value
- Messages drive all state changes — never mutate state directly

---

## Message Flow

```
Input (keyboard, mouse, resize, etc.)
        │
        ▼
   Program.dispatchMsg(msg)
        │
        ▼
  model.Update(msg)  ◄──────────────────────────┐
        │                                        │
        ├── returns (newModel, cmd)              │
        │                                        │
        ├── renderer.render(newModel.View())     │
        │                                        │
        └── if cmd != nil:                       │
               go func() {                      │
                   msg := cmd()    ─────────────┘
               }()
```

---

## Rendering Pipeline

1. `View()` returns a `tea.View` struct with `Content` string and metadata
2. Renderer diffs the new content vs previous frame
3. Only changed lines are written to stdout (efficient)
4. View metadata controls cursor, mouse mode, alt-screen, title, etc.

**Synchronized output (mode 2026):** The renderer uses terminal synchronized
output when available to prevent tearing on high-refresh displays.

**Frame rate:** Default 60 FPS. Adjustable via `WithFPS`. Rendering is
decoupled from the update loop — frames are coalesced when updates happen
faster than the frame rate.

---

## Command System

`Cmd` is defined as:
```go
type Cmd func() Msg
```

Commands are the only way to perform I/O. They run in goroutines:

```
tea.Batch(a, b, c)  →  goroutines for a, b, c run concurrently
tea.Sequence(a, b)  →  a runs, its Msg delivered, then b runs
```

**Never** close over mutable state in a Cmd — the goroutine runs after
`Update` returns, so the model may have changed.

```go
// WRONG: closes over m which may change
return m, func() tea.Msg {
    time.Sleep(m.delay)  // m.delay could be different by now
    return doneMsg{}
}

// RIGHT: capture the value at closure creation time
delay := m.delay
return m, func() tea.Msg {
    time.Sleep(delay)
    return doneMsg{}
}
```

---

## Message Type System

Messages use Go interfaces with type assertions:

```go
type Msg interface{}  // any value can be a message

// Built-in message types use structs
type KeyPressMsg struct { /* ... */ }
type WindowSizeMsg struct { Width, Height int }

// Custom messages — define your own
type dataLoadedMsg struct { items []Item }
type errorMsg       struct { err error }

// Pattern match in Update:
switch msg := msg.(type) {
case tea.KeyPressMsg:     // ...
case tea.WindowSizeMsg:   // ...
case dataLoadedMsg:       // ...
case errorMsg:            // ...
}
```

---

## Component Composition

Sub-models follow the same `Init/Update/View` pattern and are embedded
in parent models. The parent:

1. Stores child models as fields
2. Forwards relevant `Msg` values to child `Update` methods
3. Collects and batches child `Cmd` values
4. Assembles child `View` outputs using lipgloss layout

```
ParentModel
    ├── ChildA  (has Init, Update, View)
    ├── ChildB  (has Init, Update, View)
    └── state fields
```

**Message routing strategies:**

| Strategy | When to use |
|---|---|
| Broadcast to all children | `WindowSizeMsg`, global shortcuts |
| Route to focused child | Keyboard input |
| Route based on message type | Typed domain messages |
| Ignore in parent | Child-internal messages |

---

## Keyboard Enhancement Modes

BubbleTea v2 supports the Kitty keyboard protocol for enhanced input:

```go
// In View():
v.KeyboardEnhancements = tea.KeyboardEnhancements{
    SetModifyOtherKeys: tea.ModifyOtherKeysEnabled,
    // Enables: key release events, repeat distinction,
    //          unambiguous modifier combinations
}
```

Detect support:
```go
case tea.KeyboardEnhancementsMsg:
    m.hasEnhancedKeyboard = msg.SupportsKeyboardEnhancements()
```

---

## Signal Handling

Default behavior (overridable with `WithoutSignalHandler`):

| Signal | Default action |
|---|---|
| `SIGINT` (Ctrl+C) | Send `InterruptMsg` → exit with `ErrInterrupted` |
| `SIGTERM` | Send `QuitMsg` → clean exit |
| `SIGWINCH` | Send `WindowSizeMsg` |
| `SIGTSTP` (Ctrl+Z) | Send `SuspendMsg` → suspend process |
| `SIGCONT` | Send `ResumeMsg` → resume |

---

## Rendering Architecture

The renderer maintains:
- **framebuffer**: previous frame content for diffing
- **cursor position**: tracks where cursor is between frames
- **terminal state**: alt-screen, mouse mode, keyboard mode, etc.

On each frame:
1. Call `model.View()` to get new `View`
2. Diff `View.Content` against previous frame line-by-line
3. Move cursor and write only changed regions
4. Apply `View` metadata changes (mouse mode, title, etc.)

---

## Program Shutdown Sequence

1. `QuitMsg` received (from `tea.Quit`, `q` key, or SIGTERM)
2. Final `View()` rendered
3. Terminal state restored (alt-screen exited, mouse disabled, cursor restored)
4. `Run()` returns `(finalModel, nil)`

On panic (with `WithoutCatchPanics` not set):
1. Terminal restored
2. `Run()` returns `(nil, ErrProgramPanic)`
3. Panic details written to stderr
