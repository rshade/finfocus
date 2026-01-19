# Research: Cost Commands TUI Upgrade

**Feature Branch**: `106-cost-tui-upgrade`
**Date**: 2025-12-25

## Overview

This document captures research findings and technology decisions for upgrading
the cost commands to use Bubble Tea and Lip Gloss for enhanced terminal UI.

## Technology Decisions

### 1. Bubble Tea as Interactive Framework

**Decision**: Use Bubble Tea v1.2.4 for interactive TUI components

**Rationale**:

- Already a transitive dependency via `bubbles` v0.20.0
- Well-documented Elm architecture pattern (Model-Update-View)
- Native support for terminal detection and graceful degradation
- Active community with excellent examples
- Cross-platform compatibility (Linux, macOS, Windows)

**Alternatives Considered**:

- **tview**: More feature-rich but heavier; overkill for table navigation
- **termui**: Dated API, less active maintenance
- **Raw ANSI sequences**: Maximum control but significant development cost

### 2. Bubbles Table Component

**Decision**: Use `bubbles/table` for interactive resource tables

**Rationale**:

- Already wrapped in `internal/tui/table.go` with `NewTable()` helper
- Built-in keyboard navigation (up/down arrows)
- Integrates with existing `TableHeaderStyle` and `TableSelectedStyle`
- Supports dynamic row updates and height configuration

**Best Practices**:

- Pre-sort rows by cost (descending) before creating table
- Use table.WithHeight() to limit visible rows based on terminal size
- Apply `DefaultTableStyles()` for consistent styling

### 3. Output Mode Detection

**Decision**: Use existing `internal/tui/detect.go` infrastructure

**Rationale**:

- `DetectOutputMode()` already handles TTY, NO_COLOR, and CI detection
- Three-tier output: Plain → Styled → Interactive
- Respects industry-standard NO_COLOR environment variable

**Integration Pattern**:

```go
mode := tui.DetectOutputMode(forceColor, noColor, plain)
switch mode {
case tui.OutputModePlain:
    return renderPlainTable(results)
case tui.OutputModeStyled:
    return renderStyledTable(results)
case tui.OutputModeInteractive:
    return runInteractiveTUI(results)
}
```

### 4. Delta Field in CostResult

**Decision**: Add optional `Delta float64` field to `engine.CostResult`

**Rationale**:

- Spec clarification confirmed deltas come from plugins/engine
- Optional field (json:"delta,omitempty") maintains backward compatibility
- No breaking changes to existing JSON output
- `RenderDelta()` helper already exists in `internal/tui/components.go`

**Schema Change**:

```go
type CostResult struct {
    // ... existing fields ...
    Delta float64 `json:"delta,omitempty"` // Cost change from baseline
}
```

### 5. Color Scheme Consistency

**Decision**: Use existing color constants from `internal/tui/colors.go`

**Rationale**:

- Established color scheme: OK=82 (green), Warning=208 (orange), Critical=196 (red)
- Header=99 (purple), Label=245 (gray), Value=255 (white)
- Consistent with future recommendations (#216) and budget alerts (#217)
- Pre-defined styles in `internal/tui/styles.go`

**Usage Pattern**:

- Total costs: `ValueStyle`
- Cost increases: `WarningStyle` with `IconArrowUp`
- Cost decreases: `OKStyle` with `IconArrowDown`
- Headers: `HeaderStyle`

### 6. Loading State Implementation

**Decision**: Use `bubbles/spinner` with per-plugin status tracking

**Rationale**:

- `DefaultSpinner()` already available in `internal/tui/spinner.go`
- Spinner runs as sub-model within main cost view model
- Status tracking via map[string]PluginStatus for concurrent updates

**State Structure**:

```go
type LoadingState struct {
    spinner   spinner.Model
    plugins   map[string]PluginStatus // "aws" -> {Done: true, Count: 12}
    startTime time.Time
}

type PluginStatus struct {
    Done    bool
    Count   int
    Error   error
}
```

### 7. View State Management

**Decision**: Single model with view state enum

**Rationale**:

- Simpler than nested models for this use case
- Clear state transitions: Loading → List → Detail
- Easy to test state transitions in isolation

**State Machine**:

```text
┌─────────────┐
│   Loading   │ ─────────────────────────────────┐
└─────────────┘                                  │
       │                                         │
       │ plugins done                            │ timeout/error
       ▼                                         ▼
┌─────────────┐         Enter            ┌─────────────┐
│    List     │ ──────────────────────▶  │   Detail    │
└─────────────┘                          └─────────────┘
       ▲                                         │
       │               Escape                    │
       └─────────────────────────────────────────┘
```

## External Dependencies Analysis

### Bubble Tea (bubbletea v1.2.4)

- **Failure Modes**: Panic on non-TTY (handled by our mode detection)
- **Workaround**: Check `IsTTY()` before starting Bubble Tea program

### Bubbles (bubbles v0.20.0)

- **Table Component**: Stable, well-tested
- **Spinner Component**: Stable, multiple spinner styles available

### Terminal Detection (golang.org/x/term)

- **Failure Modes**: Returns error if not a terminal
- **Workaround**: `TerminalWidth()` returns DefaultTerminalWidth (80) on error

## Performance Considerations

### Resource Table Rendering

- **Target**: <100ms for 100+ resources
- **Approach**: Pre-compute styled strings, use table virtualization
- **Measurement**: Benchmark tests for table creation and navigation

### Memory Usage

- **Target**: <50MB for 1000 resources
- **Approach**: Store minimal data in table rows (strings only)
- **Measurement**: Memory profiling in integration tests

## Testing Strategy

### Unit Tests

- Model state transitions (Loading → List → Detail)
- View rendering functions (summary, table rows, detail view)
- Output mode detection edge cases

### Integration Tests

- CLI command with mock terminal environment
- JSON/NDJSON output unchanged
- Plain text fallback when TTY unavailable

### Manual Testing

- Visual verification in various terminals (iTerm2, Windows Terminal, Linux)
- Color scheme appearance in light/dark terminals
- Keyboard navigation responsiveness

## Open Questions (Resolved)

1. **Q**: Where do delta values come from?
   **A**: From `CostResult.Delta` field provided by plugins/engine (Clarification
   session 2025-12-25)

2. **Q**: Should we support mouse navigation?
   **A**: No, explicitly out of scope per spec

3. **Q**: How to handle very narrow terminals?
   **A**: Fall back to plain text output when width < 60 chars

## References

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Bubbles Component Library](https://github.com/charmbracelet/bubbles)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [NO_COLOR Standard](https://no-color.org/)
