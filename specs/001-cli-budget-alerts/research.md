# Research: Budget Status Display

This document outlines the technical decisions and research findings for the Budget Status Display feature.

## Decisions

### 1. Configuration Schema Integration
- **Decision**: Add a `CostConfig` field to the main `Config` struct in `internal/config/config.go`.
- **Rationale**: Keeps cost-related settings together. Using `yaml.v3` tags ensures compatibility with the existing configuration file.
- **Alternatives considered**: Separate `budget.yaml` file. Rejected to keep user configuration centralized in `~/.finfocus/config.yaml`.

### 2. Terminal Rendering (TTY Mode)
- **Decision**: Use `github.com/charmbracelet/lipgloss` for styling and `golang.org/x/term` for TTY detection.
- **Rationale**: `lipgloss` provides a high-level API for colors, borders, and progress bars. `x/term` is the standard Go library for terminal detection.
- **Best Practices**:
    - Detect terminal width to wrap content or truncate progress bars.
    - Use "Safe" ASCII characters for non-TTY or narrow terminals.

### 3. Forecasting Logic
- **Decision**: Linear extrapolation based on current day of the month.
- **Formula**: `forecast = (current_spend / current_day_in_period) * total_days_in_period`
- **Rationale**: Simple, easy to understand, and sufficient for an initial implementation.
- **Alternatives considered**: Moving average or seasonal models. Rejected as too complex for the first iteration and lacks enough historical data in the current architecture.

### 4. Over-budget Handling
- **Decision**: Cap progress bar at 100% width but display actual percentage in text. Use a different color (Red) for exceeded thresholds.
- **Rationale**: Prevents layout breakage while clearly communicating the overflow state.

## Integration Patterns

### CLI Output Integration
The budget status should be appended to the output of `finfocus cost` (or similar commands) when a budget is configured.
- Pattern: The CLI command should check the global config for budget settings. If present, it calls the engine to evaluate and then the CLI package to render.

## Dependencies Research

### Lip Gloss
- **Library**: `github.com/charmbracelet/lipgloss`
- **Capability**: Style strings with colors, bolding, and borders. Can create layouts using `JoinHorizontal` and `JoinVertical`.
- **Usage in project**: New dependency. Check `go.mod` after planning to ensure it's added.

### Term
- **Library**: `golang.org/x/term`
- **Capability**: `IsTerminal(fd)` to check if stdout is a TTY.
- **Usage in project**: Already used or standard.
