# Migration Quickstart: Charmbracelet v1 → v2

## Import Path Mapping

| v1 | v2 |
|----|-----|
| `github.com/charmbracelet/bubbletea` | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/bubbles/spinner` | `charm.land/bubbles/v2/spinner` |
| `github.com/charmbracelet/bubbles/table` | `charm.land/bubbles/v2/table` |
| `github.com/charmbracelet/bubbles/textinput` | `charm.land/bubbles/v2/textinput` |
| `github.com/charmbracelet/lipgloss` | `charm.land/lipgloss/v2` |

## View() Migration

```go
// v1
func (m Model) View() string {
    return content
}

// v2
func (m Model) View() tea.View {
    return tea.NewView(content)
}
```

## Key Message Migration

### Type assertion change

```go
// v1
case tea.KeyMsg:
    switch msg.String() { ... }

// v2
case tea.KeyPressMsg:
    switch msg.String() { ... }
```

### Type-field to Code-field

```go
// v1
switch msg.Type {
case tea.KeyEnter:
case tea.KeyEsc:
case tea.KeyUp:
case tea.KeyRunes:
    switch string(msg.Runes) {
    case "q": ...
    }
}

// v2
switch msg.Code {
case tea.KeyEnter:
case tea.KeyEscape:    // Note: renamed from KeyEsc
case tea.KeyUp:
default:
    if msg.Text != "" {
        switch msg.Text {
        case "q": ...
        }
    }
}
```

### Test helper migration

```go
// v1
tea.KeyMsg{Type: tea.KeyEnter}
tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
tea.KeyMsg{Type: tea.KeyCtrlC}

// v2
tea.KeyPressMsg{Code: tea.KeyEnter}
tea.KeyPressMsg{Text: "q"}
tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
```

### Space bar change

```go
// v1: msg.String() returns " "
case " ":

// v2: msg.String() returns "space"
case "space":
```

## Spinner Migration

```go
// v1
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = lipgloss.NewStyle().Foreground(color)

// v2
s := spinner.New(
    spinner.WithSpinner(spinner.Dot),
    spinner.WithStyle(lipgloss.NewStyle().Foreground(color)),
)
```

## Textinput Migration

```go
// v1
ti := textinput.New()
ti.Width = 40

// v2 — only Width changed to setter; Placeholder and CharLimit are unchanged
ti := textinput.New()
ti.SetWidth(40)
// ti.Placeholder = "..." — still works (exported field)
// ti.CharLimit = 156   — still works (exported field)
```

## Lipgloss Color Migration

```go
// v1: lipgloss.Color is a string type alias
const ColorOK = lipgloss.Color("82")

// v2: lipgloss.Color() returns color.Color interface
// May need to change const → var if color.Color is not const-compatible
var ColorOK = lipgloss.Color("82")
```

## Unchanged APIs (no migration needed)

- `tea.WindowSizeMsg` (same name and fields)
- `tea.Tick()` (same signature)
- `tea.Batch()` (same signature)
- `tea.Quit` (same value)
- `Init() tea.Cmd` (same signature)
- `table.New()` with functional options
- `table.DefaultStyles()`
- `lipgloss.NewStyle()`
- `lipgloss.JoinVertical()` / `lipgloss.Width()`
