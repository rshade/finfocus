# Research: Cost Estimate Command Implementation

**Feature**: 223-cost-estimate
**Date**: 2026-02-02
**Status**: Complete

## 1. Existing Architecture Analysis

### Decision: Follow Existing CLI Command Pattern

**Rationale**: The codebase has well-established patterns for cost commands (`cost_projected.go`, `cost_actual.go`) that should be followed for consistency.

**Alternatives Considered**:

- Custom command structure: Rejected because it would break consistency and increase maintenance burden
- Separate binary: Rejected because `cost estimate` is a natural extension of the `cost` command group

### Key Pattern Elements

```go
// 1. Parameter struct for flag values
type costEstimateParams struct {
    provider     string
    resourceType string
    properties   []string  // key=value format
    planPath     string
    modify       []string  // resource:key=value format
    interactive  bool
    output       string
    region       string
    adapter      string
}

// 2. Command constructor with RunE
func NewCostEstimateCmd() *cobra.Command {
    var params costEstimateParams
    cmd := &cobra.Command{
        Use:   "estimate",
        Short: "Estimate costs for what-if scenarios",
        RunE:  func(cmd *cobra.Command, _ []string) error { ... },
    }
    // Register flags
    return cmd
}

// 3. Use cmd.Printf() for output (not fmt.Printf)
// 4. Defer cleanup immediately after resource acquisition
// 5. Return errors early with context wrapping
```

## 2. Plugin Communication Research

### Decision: Use EstimateCost RPC with GetProjectedCost Fallback

**Rationale**: EstimateCost RPC exists in finfocus-spec v0.5.5 and provides dedicated functionality. Fallback ensures compatibility with plugins that haven't implemented it yet.

**Alternatives Considered**:

- GetProjectedCost only: Rejected because EstimateCost provides richer response (baseline, modified, deltas)
- Require EstimateCost: Rejected because existing plugins may not implement it yet

### Implementation Strategy

```go
// Try EstimateCost first
resp, err := client.API.EstimateCost(ctx, estimateReq)
if err != nil {
    if status.Code(err) == codes.Unimplemented {
        // Fallback: Call GetProjectedCost twice
        baseline, _ := client.API.GetProjectedCost(ctx, baselineReq)
        modified, _ := client.API.GetProjectedCost(ctx, modifiedReq)
        return computeDelta(baseline, modified)
    }
    return nil, err
}
return resp, nil
```

## 3. Flag Parsing Research

### Decision: Use Repeatable StringArrayVar for Properties and Modify

**Rationale**: Cobra's `StringArrayVar` allows multiple `--property key=value` flags naturally.

**Alternatives Considered**:

- Single comma-separated flag: Rejected because values may contain commas
- Config file input: Rejected as over-engineering for simple use cases

### Flag Definitions

```go
cmd.Flags().StringVar(&params.provider, "provider", "", "Cloud provider (aws, gcp, azure)")
cmd.Flags().StringVar(&params.resourceType, "resource-type", "", "Resource type (e.g., ec2:Instance)")
cmd.Flags().StringArrayVar(&params.properties, "property", nil, "Property override key=value (repeatable)")
cmd.Flags().StringVar(&params.planPath, "pulumi-json", "", "Path to Pulumi preview JSON file")
cmd.Flags().StringArrayVar(&params.modify, "modify", nil, "Resource modification resource:key=value (repeatable)")
cmd.Flags().BoolVar(&params.interactive, "interactive", false, "Launch interactive TUI mode")
cmd.Flags().StringVar(&params.output, "output", "table", "Output format (table, json, ndjson)")
cmd.Flags().StringVar(&params.region, "region", "", "Region for cost calculation")
cmd.Flags().StringVar(&params.adapter, "adapter", "", "Specific plugin adapter to use")
```

## 4. Output Formatting Research

### Decision: Extend Existing RenderResults with Delta Support

**Rationale**: Existing engine has `RenderResults()` for table/JSON/NDJSON. Extend with delta-aware rendering.

**Alternatives Considered**:

- New renderer: Rejected because existing patterns work well
- TUI-only output: Rejected because non-interactive use cases need table/JSON

### Output Structure (Table)

```text
╭───────────────────────────────────────────────────────────────────╮
│ What-If Cost Analysis                                              │
├────────────────────┬──────────────┬──────────────┬────────────────┤
│ Property           │ Current      │ Proposed     │ Monthly Δ      │
├────────────────────┼──────────────┼──────────────┼────────────────┤
│ instanceType       │ t3.micro     │ m5.large     │ +$65.70        │
│ volumeSize         │ 8 GB         │ 100 GB       │ +$9.20         │
├────────────────────┴──────────────┴──────────────┴────────────────┤
│ Baseline: $8.32/mo │ Modified: $83.22/mo │ Total Change: +$74.90  │
╰───────────────────────────────────────────────────────────────────╯
```

### Output Structure (JSON)

```json
{
  "resource": {
    "provider": "aws",
    "type": "ec2:Instance",
    "region": "us-east-1"
  },
  "baseline": {
    "monthly": 8.32,
    "currency": "USD"
  },
  "modified": {
    "monthly": 83.22,
    "currency": "USD"
  },
  "totalChange": 74.90,
  "deltas": [
    {
      "property": "instanceType",
      "originalValue": "t3.micro",
      "newValue": "m5.large",
      "costChange": 65.70
    }
  ]
}
```

## 5. TUI Research

### Decision: Extend Bubble Tea with Property Editor Model

**Rationale**: Existing TUI infrastructure uses Bubble Tea. Create new model for interactive property editing.

**Alternatives Considered**:

- Read-only TUI: Rejected because spec requires interactive editing
- External editor: Rejected as poor UX

### TUI Component Architecture

```go
type EstimateModel struct {
    // Resource context
    provider     string
    resourceType string
    region       string

    // Editable properties
    properties   []PropertyRow
    focusedRow   int
    editMode     bool
    editBuffer   string

    // Cost display
    baseline     float64
    modified     float64
    currency     string
    deltas       []CostDelta

    // State
    loading      bool
    err          error
}

type PropertyRow struct {
    Key          string
    OriginalValue string
    CurrentValue  string
    CostDelta     float64
}
```

## 6. Mutual Exclusivity Validation

### Decision: Validate in PersistentPreRunE with Clear Error Messages

**Rationale**: Fail fast with clear error messages before any processing.

**Validation Rules**:

1. Single-resource mode requires: `--provider` AND `--resource-type`
2. Plan-based mode requires: `--pulumi-json`
3. `--property` only valid in single-resource mode
4. `--modify` only valid in plan-based mode
5. Modes are mutually exclusive

```go
func validateEstimateFlags(params *costEstimateParams) error {
    hasSingleResource := params.provider != "" || params.resourceType != "" || len(params.properties) > 0
    hasPlanBased := params.planPath != "" || len(params.modify) > 0

    if hasSingleResource && hasPlanBased {
        return fmt.Errorf("cannot mix single-resource flags (--provider, --resource-type, --property) with plan-based flags (--pulumi-json, --modify)")
    }

    if hasSingleResource {
        if params.provider == "" {
            return fmt.Errorf("--provider is required for single-resource estimation")
        }
        if params.resourceType == "" {
            return fmt.Errorf("--resource-type is required for single-resource estimation")
        }
    }

    if hasPlanBased && params.planPath == "" {
        return fmt.Errorf("--pulumi-json is required for plan-based estimation")
    }

    return nil
}
```

## 7. Property Override Parsing

### Decision: Simple key=value Parsing with Validation

**Rationale**: Consistent with common CLI patterns. Validate format before processing.

```go
func parsePropertyOverrides(props []string) (map[string]string, error) {
    overrides := make(map[string]string)
    for _, p := range props {
        parts := strings.SplitN(p, "=", 2)
        if len(parts) != 2 {
            return nil, fmt.Errorf("invalid property format %q: expected key=value", p)
        }
        key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
        if key == "" {
            return nil, fmt.Errorf("property key cannot be empty in %q", p)
        }
        overrides[key] = value
    }
    return overrides, nil
}
```

## 8. Resource Modification Parsing (Plan-Based)

### Decision: resource:key=value Format with URN Support

**Rationale**: Resource names in Pulumi plans use URN format. Support both simple names and URNs.

```go
func parseModifications(mods []string) (map[string]map[string]string, error) {
    result := make(map[string]map[string]string)
    for _, m := range mods {
        // Split resource:key=value
        colonIdx := strings.Index(m, ":")
        if colonIdx == -1 {
            return nil, fmt.Errorf("invalid modify format %q: expected resource:key=value", m)
        }
        resourceName := m[:colonIdx]
        propPart := m[colonIdx+1:]

        // Parse key=value
        eqIdx := strings.Index(propPart, "=")
        if eqIdx == -1 {
            return nil, fmt.Errorf("invalid modify format %q: expected resource:key=value", m)
        }
        key := propPart[:eqIdx]
        value := propPart[eqIdx+1:]

        if result[resourceName] == nil {
            result[resourceName] = make(map[string]string)
        }
        result[resourceName][key] = value
    }
    return result, nil
}
```

## 9. Engine Method Design

### Decision: Add EstimateCost Method to Engine with Fallback Support

**Rationale**: Engine orchestrates plugin calls. New method handles estimate-specific logic including fallback.

```go
// EstimateResult represents the result of a what-if cost estimation
type EstimateResult struct {
    Resource     *ResourceDescriptor
    Baseline     *CostResult
    Modified     *CostResult
    TotalChange  float64
    Deltas       []CostDelta
    UsedFallback bool  // True if EstimateCost RPC was not available
}

type CostDelta struct {
    Property      string
    OriginalValue string
    NewValue      string
    CostChange    float64
}

// EstimateCost performs a what-if cost analysis with property overrides
func (e *Engine) EstimateCost(
    ctx context.Context,
    resource *ResourceDescriptor,
    overrides map[string]string,
) (*EstimateResult, error) {
    // Implementation with fallback logic
}
```

## 10. Testing Strategy

### Unit Tests

- Command creation and flag parsing
- Property override parsing (valid/invalid formats)
- Modification parsing (valid/invalid formats)
- Mutual exclusivity validation
- Output formatting (table, JSON, NDJSON)

### Integration Tests

- Single-resource estimation with mock plugin
- Plan-based estimation with fixture files
- Fallback behavior when EstimateCost not implemented
- Interactive mode startup (non-interactive verification)

### Test Fixtures

- `test/fixtures/estimate/single-resource.json`: Minimal Pulumi plan
- `test/fixtures/estimate/plan-with-modify.json`: Multi-resource plan for modify tests

## Summary of Key Decisions

| Topic | Decision | Rationale |
|-------|----------|-----------|
| Command Pattern | Follow existing CLI pattern | Consistency with codebase |
| RPC Strategy | EstimateCost with fallback | Compatibility + rich response |
| Flag Parsing | StringArrayVar for repeatable flags | Natural CLI UX |
| Output | Extend RenderResults | Reuse existing infrastructure |
| TUI | New Bubble Tea model | Interactive editing requirement |
| Validation | Fail fast in PersistentPreRunE | Clear error messages |
| Engine Method | New EstimateCost with fallback | Clean separation of concerns |
