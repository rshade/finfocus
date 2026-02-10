# Quickstart: Analyzer Recommendations Display

**Feature**: 106-analyzer-recommendations
**Date**: 2025-12-25

## Overview

This feature adds cost optimization recommendations to the Pulumi Analyzer
diagnostics output. When a cost plugin returns recommendations alongside
cost estimates, users will see them during `pulumi preview`.

## Prerequisites

- Go 1.25.7+
- Pulumi CLI installed
- FinFocus binary built (`make build`)
- A cost plugin that returns recommendations (e.g., aws-public with
  recommendation support)

## Building

```bash
make build
```

## Testing

### Unit Tests

```bash
# Run diagnostics tests
go test -v ./internal/analyzer/... -run TestRecommendation

# Run with coverage
go test -coverprofile=coverage.out ./internal/analyzer/...
go tool cover -func=coverage.out | grep recommendations
```

### E2E Tests

```bash
# Run E2E analyzer tests (requires Pulumi CLI and AWS credentials)
go test -v -tags=e2e ./test/e2e/... -run TestAnalyzer_Recommendation
```

## Usage

### Expected Output

When running `pulumi preview` with the analyzer enabled:

```text
Previewing update (dev):
     Type                 Name           Plan
 +   aws:ec2:Instance     webserver      create

Diagnostics:
  finfocus:cost-estimate (webserver):
    warning: Estimated Monthly Cost: $25.50 USD (source: aws-plugin) |
             Recommendations: Right-sizing: Switch to t3.small to save $15.00/mo

  finfocus:stack-cost-summary:
    warning: Total Estimated Monthly Cost: $25.50 USD (1 resource analyzed) |
             1 recommendation with $15.00/mo potential savings
```

### No Recommendations

When no recommendations are available:

```text
Diagnostics:
  finfocus:cost-estimate (webserver):
    warning: Estimated Monthly Cost: $25.50 USD (source: aws-plugin)
```

## Configuration

No additional configuration required. Recommendations are displayed
automatically when provided by cost plugins.

## Troubleshooting

### Recommendations Not Appearing

1. **Check plugin version**: Ensure the cost plugin supports the
   `GetRecommendations` RPC (pluginsdk v0.4.10+)
2. **Check plugin logs**: Enable debug logging with
   `FINFOCUS_LOG_LEVEL=debug`
3. **Verify plugin is responding**: Use `finfocus plugin validate` to
   check plugin health

### Performance Concerns

Recommendations are string-formatted during diagnostic generation with
no measurable overhead (<1ms per resource).

## Development Notes

### Key Files

- `internal/engine/types.go`: `Recommendation` struct definition
- `internal/analyzer/diagnostics.go`: `formatRecommendations()` function
- `internal/analyzer/diagnostics_test.go`: Unit tests for formatting
- `test/e2e/analyzer_e2e_test.go`: E2E validation tests

### Adding New Recommendation Types

The `Recommendation.Type` field is a free-form string. Common types:

- "Right-sizing"
- "Terminate"
- "Purchase Commitment"
- "Delete Unused"
- "Adjust Requests" (Kubernetes)

Plugins define their own types; core displays them as-is.
