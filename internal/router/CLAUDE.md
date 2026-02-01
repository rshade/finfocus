# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Router Package Overview

The `internal/router` package implements intelligent plugin routing for FinFocus cost calculations. It supports both automatic provider-based routing and declarative pattern-based routing for multi-cloud cost analysis.

## Architecture

### Core Components

1. **Router** (`router.go`)
   - Main routing engine that selects plugins based on resources and features
   - Supports automatic provider matching (AWS resources → AWS plugins)
   - Supports declarative pattern-based routing via configuration
   - Priority-based plugin selection with fallback chains

2. **Features** (`features.go`)
   - Defines valid plugin capabilities (ProjectedCosts, ActualCosts, Recommendations, etc.)
   - Maps gRPC methods to features for capability inference
   - Provides default features for plugins without explicit capability reporting

3. **Patterns** (`pattern.go`)
   - Glob and regex pattern matching for resource type routing
   - Thread-safe pattern cache for compiled regex patterns
   - Supports `filepath.Match` style globs and RE2 regex

4. **Providers** (`provider.go`)
   - Extracts provider from Pulumi resource types (e.g., `aws:ec2/instance:Instance` → `aws`)
   - Handles global plugins (empty providers or `["*"]`)
   - Case-insensitive provider matching

5. **Validation** (`validation.go`)
   - Validates routing configuration before use
   - Checks plugin existence, regex syntax, feature names, priorities
   - Distinguishes errors (blocking) from warnings (non-blocking)

### Routing Flow

```text
Resource Type → Provider Extraction → Pattern Matching → Provider Matching → Feature Filtering → Priority Sort
                                           ↓
                                     PluginMatch[]
                                           ↓
                                 First plugin tried, fallback on error
```

## Key Types

### PluginMatch

```go
type PluginMatch struct {
    Client      *pluginhost.Client  // The matched plugin
    Priority    int                 // Plugin priority (lower = higher priority)
    Fallback    bool               // Whether to try next plugin on failure
    MatchReason MatchReason        // Why this plugin matched
    Source      string             // "automatic" or "config"
}
```

### MatchReason

- `MatchReasonPattern` - Plugin matched via resource pattern
- `MatchReasonAutomatic` - Plugin matched via provider (automatic routing)

## Testing Commands

```bash
# Run router tests
go test -v ./internal/router/...

# Run with coverage
go test -cover ./internal/router/...

# Run specific test
go test -v ./internal/router/... -run TestSelectPlugins
```

## Common Usage Patterns

### Creating a Router

```go
router, err := router.NewRouter(
    router.WithClients(clients),
    router.WithConfig(routingConfig),
)
```

### Selecting Plugins

```go
matches := router.SelectPlugins(ctx, resource, "ProjectedCosts")
for _, match := range matches {
    // Try plugin, check match.Fallback to decide on error handling
}
```

### Validating Configuration

```go
result := router.ValidateRoutingConfig(cfg, clients)
if result.HasErrors() {
    for _, e := range result.Errors {
        log.Error(e.Error())
    }
}
```

## Configuration Example

```yaml
routing:
  plugins:
    - name: aws-ce
      priority: 10
      features: [ActualCosts, Recommendations]
      patterns:
        - type: glob
          pattern: "aws:*"
    - name: aws-public
      priority: 20
      features: [ProjectedCosts]
      fallback: true
```

## Pattern Matching

### Glob Patterns

- Uses `filepath.Match` semantics
- `*` matches any sequence of characters
- `?` matches any single character
- Example: `aws:ec2/*` matches `aws:ec2/instance:Instance`

### Regex Patterns

- Uses RE2 regular expressions (Go's `regexp` package)
- Compiled patterns are cached for performance
- Example: `^aws:(ec2|rds):.*` matches AWS EC2 or RDS resources

## Fallback Behavior

1. Plugins are tried in priority order (lowest priority number first)
2. If a plugin fails and `Fallback: true`, the next plugin is tried
3. If `Fallback: false`, the chain stops on failure
4. A `$0.00` cost is considered a valid result (not an empty result)
5. Empty/nil results trigger fallback to the next plugin

## Integration Points

- **Engine** (`internal/engine/`) - Uses router for plugin selection in cost calculations
- **Config** (`internal/config/`) - RoutingConfig defines the configuration schema
- **PluginHost** (`internal/pluginhost/`) - Client metadata used for provider matching
- **CLI** (`internal/cli/`) - `config validate` command uses router validation
