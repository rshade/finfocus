# Quickstart: Cost Recommendations Command Enhancement

**Feature Branch**: `109-cost-recommendations`
**Date**: 2025-12-30

## Prerequisites

- Go 1.25.7 installed
- Repository cloned and on `109-cost-recommendations` branch
- Familiarity with Bubble Tea TUI framework

## Quick Start

### 1. Checkout the Feature Branch

```bash
git checkout 109-cost-recommendations
```

### 2. Build the Project

```bash
make build
```

### 3. Run Tests

```bash
make test
```

### 4. Try Existing Command

```bash
# Basic recommendations (current behavior)
./bin/finfocus cost recommendations --pulumi-json examples/plans/aws-simple-plan.json

# With action type filter
./bin/finfocus cost recommendations --pulumi-json examples/plans/aws-simple-plan.json --filter "action=RIGHTSIZE"

# JSON output
./bin/finfocus cost recommendations --pulumi-json examples/plans/aws-simple-plan.json --output json
```

## Development Workflow

### Phase 1: Summary/Verbose Mode

**File to modify**: `internal/cli/cost_recommendations.go`

```go
// Add verbose flag to costRecommendationsParams
type costRecommendationsParams struct {
    planPath string
    adapter  string
    output   string
    filter   []string
    verbose  bool  // NEW
}

// Add flag in NewCostRecommendationsCmd
cmd.Flags().BoolVar(&params.verbose, "verbose", false, "Show all recommendations with full details")
```

**Test commands after implementation**:

```bash
# Summary mode (default - top 5)
./bin/finfocus cost recommendations --pulumi-json plan.json

# Verbose mode (all recommendations)
./bin/finfocus cost recommendations --pulumi-json plan.json --verbose
```

### Phase 2: TUI Model

**New file**: `internal/tui/recommendations_model.go`

Start with this skeleton:

```go
package tui

import (
    "github.com/charmbracelet/bubbles/table"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/rshade/finfocus/internal/engine"
)

type RecommendationsViewModel struct {
    state              ViewState
    allRecommendations []engine.Recommendation
    recommendations    []engine.Recommendation
    table              table.Model
    textInput          textinput.Model
    // ...
}

func NewRecommendationsViewModel(recs []engine.Recommendation) *RecommendationsViewModel {
    // Follow CostViewModel pattern
}

func (m *RecommendationsViewModel) Init() tea.Cmd {
    return nil
}

func (m *RecommendationsViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle keyboard input, state transitions
    return m, nil
}

func (m *RecommendationsViewModel) View() string {
    // Render based on state
    return ""
}
```

### Phase 3: TUI Views

**New file**: `internal/tui/recommendations_view.go`

```go
package tui

import (
    "github.com/charmbracelet/bubbles/table"
    "github.com/rshade/finfocus/internal/engine"
)

func NewRecommendationsTable(recs []engine.Recommendation, height int) table.Model {
    // Create table with columns: Resource, Action Type, Description, Savings
}

func RenderRecommendationsSummary(recs []engine.Recommendation, width int) string {
    // Styled summary box
}

func RenderRecommendationDetail(rec engine.Recommendation, width int) string {
    // Full recommendation details
}
```

### Phase 4: CLI Integration

**File to modify**: `internal/cli/cost_recommendations.go`

```go
func executeCostRecommendations(cmd *cobra.Command, params costRecommendationsParams) error {
    // ... existing code ...

    // Add TTY detection
    mode := tui.DetectOutputMode(false, params.noColor, params.plain)

    // Route based on mode
    switch mode {
    case tui.OutputModeInteractive:
        return runInteractiveRecommendations(filteredResult)
    case tui.OutputModeStyled:
        return renderStyledRecommendations(cmd, filteredResult, params.verbose)
    default:
        return RenderRecommendationsOutput(ctx, cmd, params.output, filteredResult)
    }
}
```

## Testing

### Run Unit Tests

```bash
go test ./internal/tui/... -v
go test ./internal/cli/... -v -run TestRecommendations
```

### Run with Coverage

```bash
go test ./internal/tui/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Manual TUI Testing

```bash
# Interactive mode (in TTY)
./bin/finfocus cost recommendations --pulumi-json plan.json

# Plain mode (bypass TTY)
./bin/finfocus cost recommendations --pulumi-json plan.json --plain

# Pipe to check plain text works
./bin/finfocus cost recommendations --pulumi-json plan.json | cat
```

## Key Patterns to Follow

### 1. Reuse Existing Styles

```go
import "github.com/rshade/finfocus/internal/tui"

// Use existing styles
tui.HeaderStyle.Render("RECOMMENDATIONS")
tui.LabelStyle.Render("Savings:")
tui.ValueStyle.Render("$123.45")
tui.BoxStyle.Width(80).Render(content)
```

### 2. Reuse ViewState

```go
// Use existing ViewState enum
tui.ViewStateLoading
tui.ViewStateList
tui.ViewStateDetail
tui.ViewStateQuitting
tui.ViewStateError
```

### 3. Reuse Keyboard Constants

```go
// Use existing constants
tui.keyEsc
tui.keyEnter
tui.keyQuit
tui.keyCtrlC
tui.keySlash
tui.keyS
```

### 4. Reuse Loading State

```go
// Use existing LoadingState
loading := tui.NewLoadingState()
tui.RenderLoading(loading)
```

## Common Issues

### Issue: TUI not starting in terminal

**Solution**: Check TTY detection:

```bash
# Force styled output (no interactivity)
./bin/finfocus cost recommendations --pulumi-json plan.json --no-color

# Check if running in TTY
tty
```

### Issue: Filter not working

**Solution**: Ensure filter format is correct:

```bash
# Correct
--filter "action=RIGHTSIZE"
--filter "action=RIGHTSIZE,TERMINATE"

# Incorrect
--filter "RIGHTSIZE"
--filter "action:RIGHTSIZE"
```

### Issue: Table too wide

**Solution**: Truncate long fields:

```go
const maxDescLen = 40
if len(description) > maxDescLen {
    description = description[:maxDescLen-3] + "..."
}
```

## Next Steps

1. Run `/speckit.tasks` to generate implementation tasks
2. Follow tasks.md for step-by-step implementation
3. Run `make lint` and `make test` before each commit
4. Update CLI help text after implementation
