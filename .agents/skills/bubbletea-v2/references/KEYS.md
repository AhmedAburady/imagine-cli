# BubbleTea v2 — Key Handling Reference

## Quick Matching Patterns

### String-based (Convenient)

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "enter":
    case "ctrl+c":
    case "q":
    case "alt+left":
    case "shift+enter":
    case "ctrl+shift+a":
    }
```

### Code-based (Type-safe)

```go
case tea.KeyPressMsg:
    key := msg.Key()
    switch key.Code {
    case tea.KeyEnter:
    case tea.KeyEscape:
    case tea.KeyF1:
    case tea.KeyBackspace:
    default:
        if len(key.Text) > 0 {
            fmt.Println("Typed:", key.Text)
        }
    }
```

### Modifier Checking

```go
case tea.KeyPressMsg:
    key := msg.Key()
    
    if key.Mod&tea.ModCtrl != 0 { /* ctrl held */ }
    if key.Mod&tea.ModAlt != 0 { /* alt/option held */ }
    if key.Mod&tea.ModShift != 0 { /* shift held */ }
    if key.Mod&tea.ModSuper != 0 { /* cmd/win held */ }
    
    if key.IsRepeat { /* key auto-repeating */ }
```

---

## Key Struct

```go
type Key struct {
    Text        string   // Printable characters (empty for special keys)
    Mod         KeyMod   // Modifier bitmask
    Code        rune     // Key constant or printable rune
    ShiftedCode rune     // Shifted variant (e.g., 'A' for shift+a)
    BaseCode    rune     // US PC-101 layout key (international keyboards)
    IsRepeat    bool     // Key is auto-repeating
}
```

### Field Details

| Field | Type | Description |
|---|---|---|
| `Code` | `rune` | Key code: `KeyEnter`, `KeyEscape`, `'a'`, etc. |
| `Text` | `string` | Printable text. Empty for special keys like Enter, Escape. |
| `Mod` | `KeyMod` | Bitmask of active modifiers |
| `ShiftedCode` | `rune` | Actual shifted key. `'A'` when pressing shift+a or caps lock on. |
| `BaseCode` | `rune` | Key on US PC-101 layout. For international keyboards, this is what the key would be on a US layout. |
| `IsRepeat` | `bool` | True when key is being held and auto-repeating (Kitty/Windows only) |

---

## Key Methods

### `String() string`

Returns human-readable representation for matching:

```go
msg.String()
// "enter", "space", "ctrl+c", "shift+enter", "alt+left"
// For printable chars: "a", "?", "A"
```

### `Keystroke() string`

Like `String()` but always includes modifier info in consistent order:

```go
key.Keystroke()
// "ctrl+shift+alt+a" (never "shift+ctrl+alt+a")
// Modifiers always in order: ctrl, alt, shift, meta, hyper, super
```

---

## Key Constants

### Basic Keys

| Constant | Description |
|---|---|
| `KeyBackspace` | Backspace |
| `KeyTab` | Tab |
| `KeyEnter` / `KeyReturn` | Enter/Return |
| `KeyEscape` / `KeyEsc` | Escape |
| `KeySpace` | Space bar |
| `KeyDelete` | Delete |

### Navigation Keys

| Constant | Description |
|---|---|
| `KeyUp`, `KeyDown`, `KeyLeft`, `KeyRight` | Arrow keys |
| `KeyHome` | Home |
| `KeyEnd` | End |
| `KeyPgUp`, `KeyPgDown` | Page Up/Down |
| `KeyBegin`, `KeyFind`, `KeyInsert`, `KeySelect` | Other navigation |

### Function Keys

| Constant | Description |
|---|---|
| `KeyF1`–`KeyF12` | Standard function keys |
| `KeyF13`–`KeyF24` | Extended function keys |
| `KeyF25`–`KeyF36` | Extended function keys |
| `KeyF37`–`KeyF48` | Extended function keys |
| `KeyF49`–`KeyF60` | Extended function keys |
| `KeyF61`–`KeyF63` | Extended function keys |

### Numpad Keys

| Constant | Description |
|---|---|
| `KeyKpEnter` | Numpad Enter |
| `KeyKpEqual`, `KeyKpMultiply`, `KeyKpPlus` | Operators |
| `KeyKpComma`, `KeyKpMinus`, `KeyKpDecimal`, `KeyKpDivide` | Operators |
| `KeyKp0`–`KeyKp9` | Numpad digits |
| `KeyKpSep` | Numpad separator |
| `KeyKpUp`, `KeyKpDown`, `KeyKpLeft`, `KeyKpRight` | Numpad arrows |
| `KeyKpPgUp`, `KeyKpPgDown`, `KeyKpHome`, `KeyKpEnd` | Numpad navigation |
| `KeyKpInsert`, `KeyKpDelete`, `KeyKpBegin` | Other numpad keys |

### Lock & Special Keys

| Constant | Description |
|---|---|
| `KeyCapsLock` | Caps Lock |
| `KeyScrollLock` | Scroll Lock |
| `KeyNumLock` | Num Lock |
| `KeyPrintScreen` | Print Screen |
| `KeyPause` | Pause/Break |
| `KeyMenu` | Menu key |

### Media Keys

| Constant | Description |
|---|---|
| `KeyMediaPlay`, `KeyMediaPause`, `KeyMediaPlayPause` | Playback controls |
| `KeyMediaReverse`, `KeyMediaStop` | Playback controls |
| `KeyMediaFastForward`, `KeyMediaRewind` | Seeking |
| `KeyMediaNext`, `KeyMediaPrev` | Track navigation |
| `KeyMediaRecord` | Record |
| `KeyLowerVol`, `KeyRaiseVol`, `KeyMute` | Volume controls |

### Modifier Keys (Left/Right)

| Constant | Description |
|---|---|
| `KeyLeftShift`, `KeyRightShift` | Shift keys |
| `KeyLeftAlt`, `KeyRightAlt` | Alt/Option keys |
| `KeyLeftCtrl`, `KeyRightCtrl` | Control keys |
| `KeyLeftSuper`, `KeyRightSuper` | Windows/Command keys |
| `KeyLeftHyper`, `KeyRightHyper` | Hyper keys |
| `KeyLeftMeta`, `KeyRightMeta` | Meta keys |
| `KeyIsoLevel3Shift`, `KeyIsoLevel5Shift` | International keyboard shift levels |

### Extended Key Code

| Constant | Description |
|---|---|
| `KeyExtended` | Special code indicating key event contains multiple runes |

---

## Modifier Bitmasks

```go
type KeyMod int

const (
    ModShift      KeyMod = 1 << iota
    ModAlt
    ModCtrl
    ModMeta
    ModHyper
    ModSuper              // Windows/Command keys
    ModCapsLock
    ModNumLock
    ModScrollLock
)
```

### Checking Modifiers

```go
key := msg.Key()

if key.Mod&tea.ModCtrl != 0 { /* ctrl held */ }
if key.Mod&tea.ModAlt != 0 { /* alt/option held */ }
if key.Mod&tea.ModShift != 0 { /* shift held */ }
if key.Mod&tea.ModSuper != 0 { /* cmd/win held */ }
if key.Mod&tea.ModMeta != 0 { /* meta held */ }
if key.Mod&tea.ModHyper != 0 { /* hyper held */ }
if key.Mod&tea.ModCapsLock != 0 { /* caps lock active */ }
if key.Mod&tea.ModNumLock != 0 { /* num lock active */ }
```

---

## Keyboard Enhancements

### Requesting Enhanced Features

```go
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.KeyboardEnhancements.ReportEventTypes = true
    return v
}
```

### KeyboardEnhancements Struct

```go
type KeyboardEnhancements struct {
    ReportEventTypes bool // Request key release + repeat events
}
```

### Detecting Support

```go
case tea.KeyboardEnhancementsMsg:
    m.hasDisambiguation = msg.SupportsKeyDisambiguation()
    m.hasEventTypes = msg.SupportsEventTypes()
```

### Features Available

| Feature | Description |
|---|---|
| **Key Disambiguation** | Distinguish `enter` vs `shift+enter`, `tab` vs `ctrl+i` |
| **Event Types** | Receive `KeyReleaseMsg` and `Key.IsRepeat` field |

---

## Message Types

### KeyPressMsg

Sent when a key is pressed:

```go
case tea.KeyPressMsg:
    fmt.Println("Pressed:", msg.String())
```

### KeyReleaseMsg

Sent when a key is released (requires keyboard enhancements):

```go
case tea.KeyReleaseMsg:
    fmt.Println("Released:", msg.String())
```

### KeyMsg (Interface)

Matches both press and release:

```go
case tea.KeyMsg:
    switch key := msg.(type) {
    case tea.KeyPressMsg:
        // key pressed
    case tea.KeyReleaseMsg:
        // key released
    }
```

---

## Common Patterns

### Quit on Multiple Keys

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "q", "ctrl+c", "esc":
        return m, tea.Quit
    }
```

### Arrow Key Navigation

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "up", "k":
        m.cursor--
    case "down", "j":
        m.cursor++
    case "left", "h":
        m.x--
    case "right", "l":
        m.x++
    }
```

### Ctrl Key Combinations

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "ctrl+s":
        m.save()
    case "ctrl+r":
        m.refresh()
    case "ctrl+q":
        return m, tea.Quit
    }
```

### Shift Key Detection

```go
case tea.KeyPressMsg:
    key := msg.Key()
    
    if key.Mod&tea.ModShift != 0 {
        switch key.Code {
        case tea.KeyUp:
            // Shift+Up: extend selection up
        case tea.KeyDown:
            // Shift+Down: extend selection down
        }
    }
```

### Key Repeat Handling

```go
case tea.KeyPressMsg:
    key := msg.Key()
    
    if key.IsRepeat {
        // Ignore auto-repeat for action keys
        return m, nil
    }
    
    // Handle initial press only
    m.handleKeyPress(msg.String())
```

### Catch-all for Printables

```go
case tea.KeyPressMsg:
    key := msg.Key()
    
    if len(key.Text) > 0 {
        m.buffer += key.Text
    }
```
