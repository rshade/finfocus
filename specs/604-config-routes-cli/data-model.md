# Data Model: Config Routes CLI Commands

**Feature**: 604-config-routes-cli
**Date**: 2026-02-28

## Existing Entities (Read-Only)

These entities already exist in the codebase and are consumed (not modified)
by the new commands.

### RoutingConfig

Source: `internal/config/routing.go`

```go
type RoutingConfig struct {
    Plugins []PluginRouting `yaml:"plugins" json:"plugins"`
}
```

- Container for all routing rules
- Accessed via `Config.Routing` (pointer, nil = automatic mode)

### PluginRouting

Source: `internal/config/routing.go`

```go
type PluginRouting struct {
    Name     string            `yaml:"name"`
    Features []string          `yaml:"features,omitempty"`
    Patterns []ResourcePattern `yaml:"patterns,omitempty"`
    Priority int               `yaml:"priority,omitempty"`
    Fallback *bool             `yaml:"fallback,omitempty"`
}
```

- `Name`: Plugin identifier (must match installed plugin name)
- `Features`: Capability filter (empty = all features allowed)
- `Patterns`: Declarative resource type patterns (empty = automatic routing)
- `Priority`: Selection precedence (higher = preferred, 0 = default)
- `Fallback`: Whether plugin serves as fallback (nil = true)

### ResourcePattern

Source: `internal/config/routing.go`

```go
type ResourcePattern struct {
    Type    string `yaml:"type"`    // "glob" or "regex"
    Pattern string `yaml:"pattern"` // The pattern string
}
```

### PluginMatch

Source: `internal/router/router.go`

```go
type PluginMatch struct {
    Client      *pluginhost.Client
    Priority    int
    Fallback    bool
    MatchReason MatchReason  // -1=NoMatch, 0=Automatic, 1=Pattern, 2=Global
    Source      string       // "automatic" or "config"
}
```

- Result of `SelectPlugins()` for a given resource type and feature
- Sorted by priority descending (higher number first)

### ResourceDescriptor

Source: `internal/engine/types.go`

```go
type ResourceDescriptor struct {
    Type       string                 `json:"type"`
    ID         string                 `json:"id"`
    Provider   string                 `json:"provider"`
    Properties map[string]interface{} `json:"properties"`
}
```

- Used as input to `SelectPlugins()`
- For `config routes test`, constructed synthetically from CLI arguments

## New Output Structures (Display-Only)

These are output-only structures for JSON serialization. They have no
persistence or lifecycle.

### RoutesListOutput

JSON output for `config routes list --output json`:

```go
type RoutesListOutput struct {
    Mode       string              `json:"mode"`        // "configured" or "automatic"
    ConfigPath string              `json:"config_path"` // Path to active config file
    Source     string              `json:"source"`      // "project" or "global"
    Rules      []RouteRuleOutput   `json:"rules"`       // Empty if mode=automatic
}

type RouteRuleOutput struct {
    Plugin   string   `json:"plugin"`
    Priority int      `json:"priority"`
    Features []string `json:"features"`  // Empty = all features
    Patterns []string `json:"patterns"`  // Formatted as "type:pattern"
    Fallback bool     `json:"fallback"`
}
```

### RoutesTestOutput

JSON output for `config routes test --output json`:

```go
type RoutesTestOutput struct {
    ResourceType string                    `json:"resource_type"`
    Region       string                    `json:"region,omitempty"`
    Provider     string                    `json:"provider"`
    Mode         string                    `json:"mode"`  // "configured" or "automatic"
    Matches      []RouteMatchOutput        `json:"matches"`
    Features     map[string]string         `json:"features"` // feature -> plugin name
}

type RouteMatchOutput struct {
    Rank        int    `json:"rank"`
    Plugin      string `json:"plugin"`
    Priority    int    `json:"priority"`
    MatchReason string `json:"match_reason"` // "pattern", "automatic", "global"
    Source      string `json:"source"`       // "config" or "automatic"
    Fallback    bool   `json:"fallback"`
}
```

## Entity Relationships

```text
Config (1) ---> (0..1) RoutingConfig
RoutingConfig (1) ---> (0..*) PluginRouting
PluginRouting (1) ---> (0..*) ResourcePattern

Router.SelectPlugins(ResourceDescriptor, feature) ---> []PluginMatch
PluginMatch (1) ---> (1) Client (synthetic, Name-only for test)
```

## State Transitions

None. Both commands are read-only. No entities are created, modified, or
deleted.
