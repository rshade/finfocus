# Data Model: Multi-Plugin Routing

**Branch**: `126-multi-plugin-routing` | **Date**: 2026-01-24

> **Note**: The `contracts/` directory contains the **authoritative** Go interface
> contracts for implementation. This data-model.md is a reference document that
> provides additional context and examples. When in doubt, `contracts/*.go` takes
> precedence.

## Entities

### 1. RoutingConfig

Top-level configuration for plugin routing.

```go
// RoutingConfig defines the complete routing strategy for plugins.
// Location: internal/config/routing.go
type RoutingConfig struct {
    // Plugins contains the ordered list of plugin routing rules.
    // Order matters for tie-breaking when priorities are equal.
    Plugins []PluginRouting `yaml:"plugins"`
}
```

**Validation Rules**:

- `Plugins` may be empty (uses automatic routing only)
- Duplicate plugin names are allowed (later entries override for overlapping patterns)

**YAML Example**:

```yaml
routing:
  plugins:
    - name: aws-ce
      features: [Recommendations]
      priority: 20
```

---

### 2. PluginRouting

Configuration for a single plugin's routing behavior.

```go
// PluginRouting defines how a specific plugin should be used.
// Location: internal/config/routing.go
type PluginRouting struct {
    // Name is the plugin identifier (must match installed plugin name).
    Name string `yaml:"name"`

    // Features limits which capabilities this plugin handles.
    // Empty means all features the plugin reports.
    // Valid: ProjectedCosts, ActualCosts, Recommendations, Carbon, DryRun, Budgets
    Features []string `yaml:"features,omitempty"`

    // Patterns defines resource type patterns this plugin handles.
    // Empty means use automatic provider-based routing.
    Patterns []ResourcePattern `yaml:"patterns,omitempty"`

    // Priority determines selection order (higher = preferred).
    // Default is 0. Equal priority means query all matching plugins.
    Priority int `yaml:"priority,omitempty"`

    // Fallback enables trying the next plugin if this one fails.
    // Default is true.
    Fallback *bool `yaml:"fallback,omitempty"`
}
```

**Validation Rules**:

- `Name` is required and must be non-empty
- `Features` must contain valid feature names (warning if invalid)
- `Priority` must be >= 0
- `Fallback` defaults to `true` if not specified

**State Transitions**: None (configuration is immutable after load)

---

### 3. ResourcePattern

Pattern for matching resource types.

```go
// ResourcePattern defines a pattern for matching resource types.
// Location: internal/config/routing.go
type ResourcePattern struct {
    // Type is the pattern type: "glob" or "regex".
    Type string `yaml:"type"`

    // Pattern is the pattern string.
    // Glob uses filepath.Match semantics.
    // Regex uses Go RE2 syntax.
    Pattern string `yaml:"pattern"`
}
```

**Validation Rules**:

- `Type` must be "glob" or "regex"
- `Pattern` must be non-empty
- If `Type` is "regex", `Pattern` must compile with `regexp.Compile`

**Examples**:

```yaml
# Glob pattern
- type: glob
  pattern: "aws:eks:*"

# Regex pattern
- type: regex
  pattern: "aws:(ec2|rds)/.*"
```

---

### 4. CompiledPattern

Internal representation of a compiled pattern.

```go
// CompiledPattern is a pre-compiled pattern for efficient matching.
// Location: internal/router/pattern.go
type CompiledPattern struct {
    // Original is the original pattern configuration.
    Original ResourcePattern

    // Regex is the compiled regex (nil for glob patterns).
    Regex *regexp.Regexp
}

// Match checks if the pattern matches the given resource type.
func (p *CompiledPattern) Match(resourceType string) (bool, error)
```

**Invariants**:

- If `Original.Type == "regex"`, then `Regex != nil`
- If `Original.Type == "glob"`, then `Regex == nil`

---

### 5. PluginMatch

Result of matching a resource to plugins.

```go
// PluginMatch represents a plugin that matches a resource.
// Location: internal/router/router.go
type PluginMatch struct {
    // Client is the matched plugin client.
    Client *pluginhost.Client

    // Priority is the configured priority (0 if not configured).
    Priority int

    // Fallback indicates if fallback is enabled for this plugin.
    Fallback bool

    // MatchReason describes why this plugin matched.
    MatchReason MatchReason
}

// MatchReason describes how a plugin was matched to a resource.
type MatchReason int

const (
    // MatchReasonAutomatic means matched via SupportedProviders.
    MatchReasonAutomatic MatchReason = iota

    // MatchReasonPattern means matched via configured pattern.
    MatchReasonPattern

    // MatchReasonGlobal means plugin is global (empty providers or "*").
    MatchReasonGlobal
)
```

---

### 6. Router

Main routing interface and implementation.

```go
// Router selects appropriate plugins for resources.
// Location: internal/router/router.go
type Router interface {
    // SelectPlugins returns matching plugins for a resource and feature.
    // Returns plugins ordered by priority (highest first).
    // If all have equal priority, all are returned for parallel query.
    SelectPlugins(ctx context.Context, resource engine.ResourceDescriptor, feature string) []PluginMatch

    // ShouldFallback checks if fallback is enabled for a plugin.
    ShouldFallback(pluginName string) bool

    // Validate performs eager validation of the routing configuration.
    Validate(ctx context.Context) ValidationResult
}

// DefaultRouter implements Router with automatic + declarative routing.
type DefaultRouter struct {
    // config is the routing configuration (may be nil for automatic-only).
    config *RoutingConfig

    // clients are all available plugin clients.
    clients []*pluginhost.Client

    // patterns is the compiled pattern cache.
    patterns map[string]*CompiledPattern

    // mu protects patterns map.
    mu sync.RWMutex
}
```

---

### 7. ValidationResult

Result of configuration validation.

```go
// ValidationResult contains the results of configuration validation.
// Location: internal/router/validation.go
type ValidationResult struct {
    // Valid is true if no errors were found.
    Valid bool

    // Errors are blocking issues that prevent routing.
    Errors []ValidationError

    // Warnings are non-blocking issues that should be reviewed.
    Warnings []ValidationWarning
}

// ValidationError represents a blocking validation error.
type ValidationError struct {
    // Plugin is the plugin name (empty for global errors).
    Plugin string

    // Field is the configuration field with the error.
    Field string

    // Message describes the error.
    Message string
}

// ValidationWarning represents a non-blocking warning.
type ValidationWarning struct {
    Plugin  string
    Field   string
    Message string
}
```

---

## Relationships

```text
RoutingConfig
    └── []PluginRouting (1:N)
           └── []ResourcePattern (1:N)
                  └── CompiledPattern (1:1, cached)

Router
    ├── RoutingConfig (1:1, optional)
    ├── []*pluginhost.Client (1:N)
    └── map[string]*CompiledPattern (pattern cache)

SelectPlugins() → []PluginMatch
    └── PluginMatch
           ├── *pluginhost.Client
           └── MatchReason
```

---

## Existing Entities (Modified)

### Config (internal/config/config.go)

Add routing field to existing Config struct.

```go
type Config struct {
    // ... existing fields ...

    // Routing configures plugin routing behavior.
    // If nil, automatic provider-based routing is used.
    Routing *RoutingConfig `yaml:"routing,omitempty"`
}
```

---

### Engine (internal/engine/engine.go)

Add router to existing Engine struct.

```go
type Engine struct {
    // ... existing fields ...

    // router selects plugins for resources.
    // If nil, queries all plugins (legacy behavior).
    router router.Router
}
```

---

## Feature Enum (Pending finfocus-spec#287)

Until the PluginCapability enum is added to finfocus-spec, use string constants:

```go
// Feature represents a plugin capability.
// Location: internal/router/features.go
type Feature string

const (
    FeatureProjectedCosts  Feature = "ProjectedCosts"
    FeatureActualCosts     Feature = "ActualCosts"
    FeatureRecommendations Feature = "Recommendations"
    FeatureCarbon          Feature = "Carbon"
    FeatureDryRun          Feature = "DryRun"
    FeatureBudgets         Feature = "Budgets"
)

// ValidFeatures returns all valid feature names.
func ValidFeatures() []Feature {
    return []Feature{
        FeatureProjectedCosts,
        FeatureActualCosts,
        FeatureRecommendations,
        FeatureCarbon,
        FeatureDryRun,
        FeatureBudgets,
    }
}

// IsValidFeature checks if a feature name is valid.
func IsValidFeature(name string) bool
```

---

## Database Schema

N/A - This feature uses file-based YAML configuration only.

---

## Configuration File Location

- **Path**: `~/.finfocus/config.yaml`
- **Format**: YAML
- **Encoding**: UTF-8
- **Permissions**: User-readable (0600 recommended)

---

## Complete Configuration Example

```yaml
# ~/.finfocus/config.yaml

# Existing plugin configuration
plugins:
  aws-public:
    region: us-east-1
  aws-ce:
    profile: cost-analysis

# NEW: Routing configuration
routing:
  plugins:
    # AWS Cost Explorer for recommendations (high priority)
    - name: aws-ce
      features:
        - Recommendations
      priority: 20
      fallback: true

    # AWS Public for projected/actual costs
    - name: aws-public
      features:
        - ProjectedCosts
        - ActualCosts
      patterns:
        - type: glob
          pattern: "aws:*"
      priority: 10
      fallback: true

    # EKS-specific plugin (highest priority for EKS resources)
    - name: eks-costs
      patterns:
        - type: regex
          pattern: "aws:eks:.*"
      priority: 30
      fallback: false

    # GCP plugin (automatic routing by provider)
    - name: gcp-public
      priority: 10
```
