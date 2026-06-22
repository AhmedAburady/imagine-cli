# BubbleTea v2 — API Reference

## Package Import

```go
import tea "charm.land/bubbletea/v2"
```

---

## Program Lifecycle

### `tea.NewProgram(model, opts...) *Program`
Creates a new program. Options are applied in order.

### `(*Program).Run() (Model, error)`
Starts the program, blocks until exit. Returns final model state.

### `(*Program).Send(msg Msg)`
Sends a message to the running program from an external goroutine.

### `(*Program).Quit()`
Quits the program from outside (for internal quitting, use `tea.Quit` command).

### `(*Program).Kill()`
Stops the program immediately without final render.

---

## Program Options

| Option | Description |
|---|---|
| `WithContext(ctx)` | Cancel program via context |
| `WithOutput(io.Writer)` | Custom output writer |
| `WithInput(io.Reader)` | Custom input reader (nil to disable) |
| `WithEnvironment([]string)` | Override environment |
| `WithFPS(fps int)` | Frame rate 1–120, default 60 |
| `WithColorProfile(p)` | Override color detection |
| `WithWindowSize(w, h int)` | Override terminal size (useful for testing) |
| `WithFilter(fn)` | Intercept/transform messages before delivery |
| `WithoutRenderer()` | Disable rendering (headless mode) |
| `WithoutSignalHandler()` | Disable default signal handling |
| `WithoutCatchPanics()` | Disable panic recovery |
| `WithoutSignals()` | Ignore OS signals (testing) |

---

## The View Type

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

### `tea.NewView(content any) View`
Create a View with the given content (string, fmt.Stringer, or Layer).

### `(*View) SetContent(content any)`
Set the content of an existing View.

---

## Cursor

```go
type Cursor struct {
    Position Position     // X, Y coordinates
    Color    color.Color  // Cursor color
    Shape    CursorShape  // Block, Underline, or Bar
    Blink    bool         // Enable blinking
}

type Position struct{ X, Y int }
```

### `tea.NewCursor(x, y int) *Cursor`
Create a cursor at the specified position.

### Cursor Shapes

| Constant | Description |
|---|---|
| `CursorBlock` | Block cursor |
| `CursorUnderline` | Underline cursor |
| `CursorBar` | I-beam / bar cursor |

---

## Mouse Modes

| Constant | Description |
|---|---|
| `MouseModeNone` | No mouse events |
| `MouseModeCellMotion` | Clicks, drags, wheel |
| `MouseModeAllMotion` | All movement events (high traffic) |

---

## Progress Bar

```go
type ProgressBar struct {
    State ProgressBarState
    Value int // 0-100
}
```

### `tea.NewProgressBar(state ProgressBarState, value int) *ProgressBar`

### Progress Bar States

| Constant | Description |
|---|---|
| `ProgressBarNone` | No progress bar |
| `ProgressBarDefault` | Normal progress |
| `ProgressBarError` | Error state (red) |
| `ProgressBarIndeterminate` | Unknown progress (spinner) |
| `ProgressBarWarning` | Warning state (yellow) |

---

## Keyboard Enhancements

```go
type KeyboardEnhancements struct {
    ReportEventTypes bool // Request key release + repeat events
}
```

---

## Message Types

### Keyboard

| Type | Fields | Notes |
|---|---|
| `KeyPressMsg` | `Key() Key`, `String() string` | Key down |
| `KeyReleaseMsg` | `Key() Key`, `String() string` | Key up (enhanced mode only) |
| `KeyMsg` | Interface | Both press and release |

### Key Struct

```go
type Key struct {
    Text        string   // Printable characters (empty for special keys)
    Mod         KeyMod   // Modifier bitmask
    Code        rune     // Key constant or printable rune
    ShiftedCode rune     // Shifted variant (Kitty/Windows only)
    BaseCode    rune     // US PC-101 layout key (international keyboards)
    IsRepeat    bool     // Key is auto-repeating (Kitty/Windows only)
}
```

**Key methods:**
- `String() string` - Human-readable: `"enter"`, `"ctrl+c"`, `"a"`
- `Keystroke() string` - Always includes modifiers: `"ctrl+shift+a"`

### Key Constants

**Basic:** `KeyEnter`, `KeyEscape`, `KeyBackspace`, `KeyDelete`, `KeyTab`, `KeySpace`

**Navigation:** `KeyUp`, `KeyDown`, `KeyLeft`, `KeyRight`, `KeyHome`, `KeyEnd`, `KeyPgUp`, `KeyPgDown`

**Function:** `KeyF1`–`KeyF63`

**Numpad:** `KeyKp0`–`KeyKp9`, `KeyKpEnter`, `KeyKpPlus`, `KeyKpMinus`, `KeyKpMultiply`, `KeyKpDivide`

**Lock:** `KeyCapsLock`, `KeyScrollLock`, `KeyNumLock`

**Media:** `KeyMediaPlay`, `KeyMediaPause`, `KeyMediaNext`, `KeyMediaPrev`, `KeyLowerVol`, `KeyRaiseVol`, `KeyMute`

**Modifiers:** `KeyLeftShift`, `KeyRightShift`, `KeyLeftCtrl`, `KeyRightCtrl`, `KeyLeftAlt`, `KeyRightAlt`, `KeyLeftSuper`, `KeyRightSuper`

### Modifier Bitmasks

```go
const (
    ModShift      // Shift key
    ModAlt        // Alt/Option key
    ModCtrl       // Control key
    ModMeta       // Meta key
    ModSuper      // Windows/Command key
    ModHyper      // Hyper key
    ModCapsLock   // Caps Lock active
    ModNumLock    // Num Lock active
    ModScrollLock // Scroll Lock active
)
```

### Mouse

| Type | Fields |
|---|---|
| `MouseClickMsg` | `X, Y int`, `Button MouseButton`, `Mod KeyMod` |
| `MouseReleaseMsg` | `X, Y int`, `Button MouseButton`, `Mod KeyMod` |
| `MouseWheelMsg` | `X, Y int`, `Button MouseButton`, `Mod KeyMod` |
| `MouseMotionMsg` | `X, Y int`, `Button MouseButton`, `Mod KeyMod` |

All implement `MouseMsg` interface: `Mouse() Mouse`.

### Mouse Struct

```go
type Mouse struct {
    X, Y   int
    Button MouseButton
    Mod    KeyMod
}
```

### Mouse Buttons

| Constant | Description |
|---|---|
| `MouseNone` | No button |
| `MouseLeft` | Left button |
| `MouseMiddle` | Middle button (scroll wheel click) |
| `MouseRight` | Right button |
| `MouseWheelUp`, `MouseWheelDown` | Scroll wheel |
| `MouseWheelLeft`, `MouseWheelRight` | Horizontal scroll |
| `MouseBackward`, `MouseForward` | Browser back/forward buttons |
| `MouseButton10`, `MouseButton11` | Additional buttons |

### Window & Focus

| Type | Fields |
|---|---|
| `WindowSizeMsg` | `Width, Height int` |
| `FocusMsg` | Terminal gained focus |
| `BlurMsg` | Terminal lost focus |

### Terminal Color Queries

| Type | Method / Fields |
|---|---|
| `BackgroundColorMsg` | `IsDark() bool`, `Color() color.Color` |
| `ForegroundColorMsg` | `IsDark() bool`, `Color() color.Color` |
| `CursorColorMsg` | `IsDark() bool`, `Color() color.Color` |
| `ColorProfileMsg` | `Profile colorprofile.Profile` |

### Paste

| Type | Description |
|---|---|
| `PasteStartMsg` | Bracketed paste starts |
| `PasteMsg` | `Content string` - pasted text |
| `PasteEndMsg` | Bracketed paste ends |

### Clipboard (OSC52)

| Type | Description |
|---|---|
| `ClipboardMsg` | `Content string`, `Clipboard() byte` - 'c' for system, 'p' for primary |
| `PrimaryClipboardMsg` | Primary selection contents |

### Terminal Queries

| Type | Fields |
|---|---|
| `TerminalVersionMsg` | `Name string` - terminal name |
| `CursorPositionMsg` | `X, Y int` |
| `CapabilityMsg` | `Content string` |
| `ModeReportMsg` | `Mode ansi.Mode`, `Value ansi.ModeSetting` |
| `EnvMsg` | Environment variables (SSH sessions) |

### Keyboard Enhancements

| Type | Methods |
|---|---|
| `KeyboardEnhancementsMsg` | `SupportsKeyDisambiguation() bool`, `SupportsEventTypes() bool` |

### Program Control

| Type | Description |
|---|---|
| `QuitMsg` | Graceful exit |
| `InterruptMsg` | Ctrl+C |
| `SuspendMsg` | Ctrl+Z suspend |
| `ResumeMsg` | Resume after suspend |
| `BatchMsg` | Internal: concurrent commands |

---

## Commands (Cmd)

A `Cmd` is `func() Msg` — runs in a goroutine and sends its return value as a message.

### Lifecycle

```go
tea.Quit        // var Cmd — send QuitMsg
tea.Interrupt   // var Msg — send InterruptMsg
tea.Suspend     // var Msg — send SuspendMsg
tea.ClearScreen // var Msg — clear and reset cursor
```

### Composition

```go
tea.Batch(cmds ...Cmd) Cmd    // run all concurrently
tea.Sequence(cmds ...Cmd) Cmd // run in order, each waits for previous
```

### Timers

```go
tea.Tick(d time.Duration, fn func(time.Time) Msg) Cmd   // One-time tick
tea.Every(d time.Duration, fn func(time.Time) Msg) Cmd  // Aligned to clock
```

### Terminal Queries

```go
tea.RequestWindowSize() Msg
tea.RequestBackgroundColor() Msg
tea.RequestForegroundColor() Msg
tea.RequestCursorColor() Msg
tea.RequestCursorPosition() Msg
tea.RequestTerminalVersion() Msg
tea.RequestCapability(s string) Msg
```

### Output

```go
tea.Printf(format string, args ...any) Cmd  // print above program
tea.Println(args ...any) Cmd
tea.Raw(seq any) Cmd                        // raw ANSI escape sequence
```

### Clipboard

```go
tea.SetClipboard(s string) Cmd
tea.ReadClipboard() Msg                     // returns ClipboardMsg
tea.SetPrimaryClipboard(s string) Cmd
tea.ReadPrimaryClipboard() Msg
```

### External Processes

```go
tea.Exec(cmd ExecCommand, onExit func(error) Msg) Cmd
tea.ExecProcess(cmd *exec.Cmd, onExit func(error) Msg) Cmd
```

---

## Errors

```go
tea.ErrInterrupted    // Ctrl+C / InterruptMsg received
tea.ErrProgramKilled  // Program killed externally
tea.ErrProgramPanic   // Panic was caught
```

---

## Logging

```go
tea.LogToFile(path, prefix string) (*os.File, error)
tea.LogToFileWith(path, prefix string, logger LogOptionsSetter) (*os.File, error)
```

Environment variables:
- `TEA_TRACE=<path>` — write trace log to path
- `TEA_DEBUG=true` — enable debug output

---

## Advanced Types

### Layer Interface

```go
type Layer interface {
    Draw(s Screen, r Rectangle)
}
```

### Hittable Interface

```go
type Hittable interface {
    Hit(x, y int) string
}
```

### EnvMsg Methods

```go
msg.Getenv(key string) string
msg.LookupEnv(key string) (string, bool)
```

---

## Related Packages

| Package | Purpose |
|---|---|
| `charm.land/lipgloss/v2` | ANSI styles, colors, layout |
| `charm.land/bubbles/v2` | Reusable components (spinner, list, input, viewport…) |
| `github.com/charmbracelet/x/ansi` | ANSI utilities |
| `github.com/charmbracelet/colorprofile` | Terminal color capability detection |

---

## Detailed References

For comprehensive coverage of specific topics:

- **Key handling** → [KEYS.md](KEYS.md)
- **Terminal features** → [TERMINAL-FEATURES.md](TERMINAL-FEATURES.md)
- **View features** → [VIEW-FEATURES.md](VIEW-FEATURES.md)
