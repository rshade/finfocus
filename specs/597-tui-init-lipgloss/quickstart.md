# Quickstart: TUI Initializing View Lipgloss Consistency

**Branch**: `597-tui-init-lipgloss`
**Date**: 2026-02-18

## What This Change Does

Replaces the raw `fmt.Sprintf("\n %s %s\n\n", spinnerView, msg)` call in
`renderInitializingView()` with lipgloss-styled output that:

- Renders the message text with `InfoStyle` (blue, bold)
- Constrains width to `m.width - borderPadding` (terminal-width-aware)
- Applies `Padding(1, 2)` for visual consistency with other full-screen states
- Composes spinner and message horizontally with `lipgloss.JoinHorizontal`

## Files Changed

| File | Change |
|------|--------|
| `internal/tui/overview_view.go` | Replace `renderInitializingView` body (lines 35–45) |
| `internal/tui/overview_view_test.go` | Add 2 new tests; existing tests continue to pass |

## Before / After

**Before** (plain text, no width awareness):

```go
return fmt.Sprintf("\n %s %s\n\n", spinnerView, msg)
```

**After** (lipgloss-styled, width-responsive):

```go
content := lipgloss.JoinHorizontal(lipgloss.Left,
    spinnerView,
    " ",
    InfoStyle.Render(msg),
)
return lipgloss.NewStyle().
    Width(m.width - borderPadding).
    Padding(1, 2).
    Render(content)
```

## Running Tests

```bash
go test -v ./internal/tui/... -run TestOverviewView_Initializing
make test
make lint
```

## Notes

- The `fmt` import in `overview_view.go` is NOT removed — it is still used by
  `renderStatusBar`, `renderDetailCostDrift`, and other functions.
- `borderPadding = 2` lives in `cost_view.go` and is shared across the `tui` package.
