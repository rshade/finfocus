# Quickstart: Cost Estimate Command Development

**Feature**: 223-cost-estimate
**Date**: 2026-02-02

## Prerequisites

- Go 1.25.6 installed
- finfocus repository cloned
- Make and golangci-lint available

## Development Setup

```bash
# Ensure you're on the feature branch
git checkout 223-cost-estimate

# Install dependencies
go mod download

# Verify build
make build

# Run tests
make test

# Run linter
make lint
```

## Key Files to Implement

### 1. CLI Command (Start Here)

Create `internal/cli/cost_estimate.go`:

```go
package cli

import (
    "github.com/spf13/cobra"
)

type costEstimateParams struct {
    provider     string
    resourceType string
    properties   []string
    planPath     string
    modify       []string
    interactive  bool
    output       string
    region       string
    adapter      string
}

func NewCostEstimateCmd() *cobra.Command {
    var params costEstimateParams

    cmd := &cobra.Command{
        Use:   "estimate",
        Short: "Estimate costs for what-if scenarios",
        Long: `Perform what-if cost analysis on resources without modifying Pulumi code.

Supports two modes:
  - Single-resource: Specify provider, type, and property overrides
  - Plan-based: Load a Pulumi plan and apply modifications

Examples:
  # Single resource estimation
  finfocus cost estimate --provider aws --resource-type ec2:Instance \
    --property instanceType=m5.large

  # Plan-based estimation
  finfocus cost estimate --pulumi-json plan.json \
    --modify "web-server:instanceType=m5.large"

  # Interactive mode
  finfocus cost estimate --interactive`,
        RunE: func(cmd *cobra.Command, _ []string) error {
            return executeCostEstimate(cmd, params)
        },
    }

    // Register flags
    cmd.Flags().StringVar(&params.provider, "provider", "", "Cloud provider (aws, gcp, azure)")
    cmd.Flags().StringVar(&params.resourceType, "resource-type", "", "Resource type (e.g., ec2:Instance)")
    cmd.Flags().StringArrayVar(&params.properties, "property", nil, "Property override key=value (repeatable)")
    cmd.Flags().StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON file")
    cmd.Flags().StringArrayVar(&params.modify, "modify", nil, "Resource modification resource:key=value (repeatable)")
    cmd.Flags().BoolVar(&params.interactive, "interactive", false, "Launch interactive TUI mode")
    cmd.Flags().StringVar(&params.output, "output", "table", "Output format (table, json, ndjson)")
    cmd.Flags().StringVar(&params.region, "region", "", "Region for cost calculation")
    cmd.Flags().StringVar(&params.adapter, "adapter", "", "Specific plugin adapter to use")

    return cmd
}

func executeCostEstimate(cmd *cobra.Command, params costEstimateParams) error {
    // TODO: Implement - see research.md for patterns
    return nil
}
```

### 2. Register Command

Add to `internal/cli/cost.go`:

```go
func newCostCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "cost",
        Short: "Cost analysis commands",
    }

    // Existing commands
    cmd.AddCommand(NewCostProjectedCmd())
    cmd.AddCommand(NewCostActualCmd())
    cmd.AddCommand(NewCostRecommendationsCmd())

    // NEW: Add estimate command
    cmd.AddCommand(NewCostEstimateCmd())

    return cmd
}
```

### 3. Engine Method

Add to `internal/engine/engine.go`:

```go
// EstimateCost performs what-if cost analysis with property overrides
func (e *Engine) EstimateCost(
    ctx context.Context,
    resource *ResourceDescriptor,
    overrides map[string]string,
) (*EstimateResult, error) {
    // TODO: Implement - see research.md for patterns
    return nil, nil
}
```

### 4. Types

Add to `internal/engine/types.go`:

```go
// EstimateResult represents the result of a what-if cost estimation
type EstimateResult struct {
    Resource     *ResourceDescriptor
    Baseline     *CostResult
    Modified     *CostResult
    TotalChange  float64
    Deltas       []CostDelta
    UsedFallback bool
}

// CostDelta represents the cost impact of a single property change
type CostDelta struct {
    Property      string
    OriginalValue string
    NewValue      string
    CostChange    float64
}
```

## Testing Workflow

### Run Unit Tests

```bash
# Run all tests
go test ./internal/cli/... ./internal/engine/...

# Run specific test file
go test -v ./internal/cli/cost_estimate_test.go

# Run with coverage
go test -coverprofile=coverage.out ./internal/cli/...
go tool cover -html=coverage.out
```

### Create Test Fixtures

Create `test/fixtures/estimate/single-resource.json`:

```json
{
  "version": 3,
  "resources": [
    {
      "type": "aws:ec2/instance:Instance",
      "name": "test-server",
      "inputs": {
        "instanceType": "t3.micro",
        "ami": "ami-12345678"
      }
    }
  ]
}
```

### Manual Testing

```bash
# Build the binary
make build

# Test single-resource mode
./bin/finfocus cost estimate \
  --provider aws \
  --resource-type ec2:Instance \
  --property instanceType=m5.large \
  --output json

# Test plan-based mode
./bin/finfocus cost estimate \
  --pulumi-json test/fixtures/estimate/single-resource.json \
  --modify "test-server:instanceType=m5.large"

# Test help
./bin/finfocus cost estimate --help
```

## Common Patterns

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("loading plan: %w", err)
}

// Use cmd.Printf for output
cmd.Printf("Baseline: $%.2f/mo\n", result.Baseline.Monthly)
```

### Logging

```go
log := logging.FromContext(ctx)
log.Debug().
    Str("provider", params.provider).
    Str("resource_type", params.resourceType).
    Msg("starting cost estimation")
```

### Testify Assertions

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCostEstimate_SingleResource(t *testing.T) {
    // Setup that must succeed
    require.NoError(t, err)

    // Value assertions
    assert.Equal(t, expected, actual)
    assert.Contains(t, output.String(), "Monthly")
}
```

## Checklist

- [ ] Create `internal/cli/cost_estimate.go`
- [ ] Create `internal/cli/cost_estimate_test.go`
- [ ] Add types to `internal/engine/types.go`
- [ ] Create `internal/engine/estimate.go`
- [ ] Create `internal/engine/estimate_test.go`
- [ ] Register command in `internal/cli/cost.go`
- [ ] Create test fixtures
- [ ] Run `make lint` - must pass
- [ ] Run `make test` - must pass with 80%+ coverage
- [ ] Manual testing of all modes

## Reference Documents

- [spec.md](./spec.md) - Feature specification
- [research.md](./research.md) - Architecture research
- [data-model.md](./data-model.md) - Data structures
- [contracts/estimate-rpc.md](./contracts/estimate-rpc.md) - gRPC contract
