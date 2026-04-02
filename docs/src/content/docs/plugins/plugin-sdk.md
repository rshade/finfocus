---
title: Plugin SDK Reference
description: Complete API reference for the FinFocus Plugin SDK
---

This document provides complete API reference for the FinFocus Plugin SDK
(`github.com/rshade/finfocus-spec/sdk/go/pluginsdk`). The SDK simplifies plugin development by providing
interfaces, helper functions, and utilities for building cost source plugins.

## Table of Contents

1. [Core Interfaces](#core-interfaces)
2. [Server and Serving](#server-and-serving)
3. [Helper Types](#helper-types)
4. [Manifest Management](#manifest-management)
5. [Testing Utilities](#testing-utilities)
6. [Code Examples](#code-examples)

---

## Core Interfaces

### Plugin Interface

The `Plugin` interface defines the contract for all FinFocus plugins.

```go
type Plugin interface {
    // Name returns the plugin name identifier.
    Name() string

    // GetProjectedCost calculates projected cost for a resource.
    GetProjectedCost(
        ctx context.Context,
        req *pbc.GetProjectedCostRequest,
    ) (*pbc.GetProjectedCostResponse, error)

    // GetActualCost retrieves actual cost for a resource.
    GetActualCost(
        ctx context.Context,
        req *pbc.GetActualCostRequest,
    ) (*pbc.GetActualCostResponse, error)
}
```

**Methods:**

- **`Name() string`**
  - Returns the plugin's unique identifier
  - Must be lowercase, alphanumeric with hyphens
  - Example: `"kubecost"`, `"vantage-api"`

- **`GetProjectedCost(ctx, req) (resp, error)`**
  - Calculates projected monthly cost for a resource
  - Must return cost in specified currency
  - Required to implement plugin interface

- **`GetActualCost(ctx, req) (resp, error)`**
  - Retrieves historical cost data
  - Returns time-series cost data
  - Can return `NoDataError` if not implemented

---

## Server and Serving

### Server Type

The `Server` wraps a `Plugin` implementation with gRPC handling.

```go
type Server struct {
    pbc.UnimplementedCostSourceServiceServer
    plugin Plugin
}
```

**Constructor:**

```go
func NewServer(plugin Plugin) *Server
```

Creates a new gRPC server wrapper for the provided plugin.

**Methods:**

- **`Name(ctx, req) (*NameResponse, error)`**
  - Implements the gRPC Name RPC
  - Delegates to `plugin.Name()`

- **`GetProjectedCost(ctx, req) (*GetProjectedCostResponse, error)`**
  - Implements the gRPC GetProjectedCost RPC
  - Delegates to `plugin.GetProjectedCost()`

- **`GetActualCost(ctx, req) (*GetActualCostResponse, error)`**
  - Implements the gRPC GetActualCost RPC
  - Delegates to `plugin.GetActualCost()`

### ServeConfig

Configuration for serving a plugin.

```go
type ServeConfig struct {
    Plugin Plugin // Plugin implementation to serve
    Port   int    // Port number (0 = auto-select)
}
```

**Fields:**

- **`Plugin`** - The plugin implementation to serve
- **`Port`** - TCP port (0 uses PORT env var or ephemeral port)

### Serve Function

Starts the gRPC server for a plugin.

```go
func Serve(ctx context.Context, config ServeConfig) error
```

**Parameters:**

- `ctx` - Context for graceful shutdown
- `config` - Server configuration

**Behavior:**

1. Resolves port (config.Port → PORT env → ephemeral)
2. Creates TCP listener on 127.0.0.1
3. Prints `PORT=<number>` to stdout
4. Registers plugin as gRPC service
5. Serves until context is cancelled
6. Performs graceful shutdown

**Returns:**

- `nil` on clean shutdown
- Error if server fails to start or serve

**Example:**

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

err := pluginsdk.Serve(ctx, pluginsdk.ServeConfig{
    Plugin: myPlugin,
    Port:   0,
})
if err != nil {
    log.Fatal(err)
}
```

---

## Helper Types

### BasePlugin

Provides common functionality for plugin implementations.

```go
type BasePlugin struct {
    // Private fields
}
```

**Constructor:**

```go
func NewBasePlugin(name string) *BasePlugin
```

Creates a base plugin with initialized matcher and calculator.

**Methods:**

- **`Name() string`**
  - Returns the plugin name

- **`Matcher() *ResourceMatcher`**
  - Returns the resource matcher for configuration

- **`Calculator() *CostCalculator`**
  - Returns the cost calculator for helpers

- **`GetProjectedCost(ctx, req) (resp, error)`**
  - Default implementation returning `NotSupportedError`
  - Plugins should override this method

- **`GetActualCost(ctx, req) (resp, error)`**
  - Default implementation returning `NoDataError`
  - Plugins should override this method

**Example:**

```go
type MyPlugin struct {
    *pluginsdk.BasePlugin
}

func NewMyPlugin() *MyPlugin {
    base := pluginsdk.NewBasePlugin("my-plugin")
    base.Matcher().AddProvider("aws")
    return &MyPlugin{BasePlugin: base}
}
```

### ResourceMatcher

Helps determine if a plugin supports a resource.

```go
type ResourceMatcher struct {
    // Private fields
}
```

**Constructor:**

```go
func NewResourceMatcher() *ResourceMatcher
```

**Methods:**

- **`AddProvider(provider string)`**
  - Adds a supported cloud provider
  - Examples: `"aws"`, `"azure"`, `"gcp"`, `"kubernetes"`

- **`AddResourceType(resourceType string)`**
  - Adds a supported resource type
  - Examples: `"aws:ec2:Instance"`, `"azure:compute:VirtualMachine"`

- **`Supports(resource *ResourceDescriptor) bool`**
  - Checks if the resource is supported
  - Returns true if provider and type match

**Example:**

```go
matcher := pluginsdk.NewResourceMatcher()
matcher.AddProvider("aws")
matcher.AddProvider("azure")
matcher.AddResourceType("aws:ec2:Instance")
matcher.AddResourceType("aws:rds:Instance")

if matcher.Supports(resource) {
    // Calculate cost
}
```

### CostCalculator

Provides utilities for cost calculations and responses.

```go
type CostCalculator struct{}
```

**Constants:**

- **`hoursPerMonth = 730.0`** - Used for monthly cost calculations

**Constructor:**

```go
func NewCostCalculator() *CostCalculator
```

**Methods:**

- **`HourlyToMonthly(hourlyCost float64) float64`**
  - Converts hourly cost to monthly (× 730)

- **`MonthlyToHourly(monthlyCost float64) float64`**
  - Converts monthly cost to hourly (÷ 730)

- **`CreateProjectedCostResponse(currency, unitPrice, billingDetail)`**
  - Creates a standard projected cost response
  - `unitPrice` is the hourly rate
  - Automatically calculates `CostPerMonth`

- **`CreateActualCostResponse(results []*ActualCostResult)`**
  - Creates a standard actual cost response
  - Wraps the provided cost results

**Example:**

```go
calc := pluginsdk.NewCostCalculator()

// Convert costs
monthlyRate := calc.HourlyToMonthly(0.0104)  // 7.592

// Create response
resp := calc.CreateProjectedCostResponse(
    "USD",
    0.0104,
    "on-demand pricing",
)
// resp.CostPerMonth == 7.592
```

---

## Manifest Management

### Manifest Type

Represents plugin metadata.

```go
type Manifest struct {
    Name               string            `yaml:"name" json:"name"`
    Version            string            `yaml:"version" json:"version"`
    Description        string            `yaml:"description" json:"description"`
    Author             string            `yaml:"author" json:"author"`
    SupportedProviders []string          `yaml:"supported_providers"`
    Protocols          []string          `yaml:"protocols"`
    Binary             string            `yaml:"binary" json:"binary"`
    Metadata           map[string]string `yaml:"metadata,omitempty"`
}
```

**Fields:**

- **`Name`** - Plugin name (lowercase, alphanumeric with hyphens)
- **`Version`** - Semantic version (e.g., "1.0.0")
- **`Description`** - Human-readable description
- **`Author`** - Author or organization name
- **`SupportedProviders`** - List of cloud providers
- **`Protocols`** - Communication protocols (always `["grpc"]`)
- **`Binary`** - Path to plugin executable
- **`Metadata`** - Additional key-value metadata

**Methods:**

- **`Validate() error`**
  - Validates all manifest fields
  - Returns `ValidationErrors` with all issues

- **`SaveManifest(path string) error`**
  - Saves manifest to YAML or JSON file
  - Format determined by file extension

**Functions:**

- **`LoadManifest(path string) (*Manifest, error)`**
  - Loads and validates manifest from file
  - Supports `.yaml`, `.yml`, `.json` extensions

- **`CreateDefaultManifest(name, author, providers) *Manifest`**
  - Creates manifest with sensible defaults
  - Version set to "1.0.0"
  - Protocols set to `["grpc"]`

**Example:**

```go
manifest := pluginsdk.CreateDefaultManifest(
    "my-plugin",
    "John Doe",
    []string{"aws", "azure"},
)

manifest.Description = "My custom cost plugin"

err := manifest.SaveManifest("plugin.manifest.yaml")
if err != nil {
    log.Fatal(err)
}
```

### ValidationError

Represents a single manifest validation error.

```go
type ValidationError struct {
    Field   string
    Message string
}
```

**Methods:**

- **`Error() string`**
  - Returns formatted error message

### ValidationErrors

Represents multiple validation errors.

```go
type ValidationErrors []ValidationError
```

**Methods:**

- **`Error() string`**
  - Returns formatted multi-line error message
  - Includes count and details of all errors

---

## Testing Utilities

The SDK provides testing utilities in `testing.go`.

### Test Helpers

**Functions:**

- **`CreateTestResourceDescriptor(provider, resourceType, sku)`**
  - Creates a resource descriptor for testing
  - Includes common test tags

- **`AssertProjectedCost(t, response, expectedCurrency, expectedUnitPrice)`**
  - Asserts projected cost response values
  - Automatically fails test on mismatch

**Example:**

```go
func TestMyPlugin(t *testing.T) {
    plugin := NewMyPlugin()

    resource := pluginsdk.CreateTestResourceDescriptor(
        "aws",
        "aws:ec2:Instance",
        "t3.micro",
    )

    req := &pbc.GetProjectedCostRequest{Resource: resource}
    resp, err := plugin.GetProjectedCost(context.Background(), req)

    require.NoError(t, err)
    pluginsdk.AssertProjectedCost(t, resp, "USD", 0.0104)
}
```

---

## Helper Functions

### Error Helpers

Standard error constructors for common scenarios.

**Functions:**

- **`NotSupportedError(resource *ResourceDescriptor) error`**
  - Returns error indicating resource is not supported
  - Includes resource type and provider in message

- **`NoDataError(resourceID string) error`**
  - Returns error indicating no cost data available
  - Includes resource ID in message

**Example:**

```go
func (p *MyPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()

    if !p.Matcher().Supports(resource) {
        return nil, pluginsdk.NotSupportedError(resource)
    }

    // Calculate cost...
}

func (p *MyPlugin) GetActualCost(
    ctx context.Context,
    req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
    // If historical data not available
    return nil, pluginsdk.NoDataError(req.GetResourceId())
}
```

---

## Code Examples

### Minimal Plugin Implementation

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type MinimalPlugin struct {
    *pluginsdk.BasePlugin
}

func NewMinimalPlugin() *MinimalPlugin {
    base := pluginsdk.NewBasePlugin("minimal")
    base.Matcher().AddProvider("aws")
    base.Matcher().AddResourceType("aws:ec2:Instance")
    return &MinimalPlugin{BasePlugin: base}
}

func (p *MinimalPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()

    if !p.Matcher().Supports(resource) {
        return nil, pluginsdk.NotSupportedError(resource)
    }

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        0.0104,
        "fixed-rate",
    ), nil
}

func main() {
    plugin := NewMinimalPlugin()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigChan
        cancel()
    }()

    config := pluginsdk.ServeConfig{
        Plugin: plugin,
        Port:   0,
    }

    log.Printf("Starting %s plugin...", plugin.Name())
    if err := pluginsdk.Serve(ctx, config); err != nil {
        log.Fatalf("Failed to serve: %v", err)
    }
}
```

### Multi-Provider Plugin

```go
type MultiProviderPlugin struct {
    *pluginsdk.BasePlugin
    awsPrices   map[string]float64
    azurePrices map[string]float64
}

func NewMultiProviderPlugin() *MultiProviderPlugin {
    base := pluginsdk.NewBasePlugin("multi-provider")

    // Support multiple providers
    base.Matcher().AddProvider("aws")
    base.Matcher().AddProvider("azure")

    // Support multiple resource types
    base.Matcher().AddResourceType("aws:ec2:Instance")
    base.Matcher().AddResourceType("azure:compute:VirtualMachine")

    return &MultiProviderPlugin{
        BasePlugin: base,
        awsPrices: map[string]float64{
            "t3.micro": 0.0104,
            "t3.small": 0.0208,
        },
        azurePrices: map[string]float64{
            "Standard_B1s": 0.0104,
            "Standard_B2s": 0.0416,
        },
    }
}

func (p *MultiProviderPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()

    if !p.Matcher().Supports(resource) {
        return nil, pluginsdk.NotSupportedError(resource)
    }

    var price float64
    var detail string

    switch resource.GetProvider() {
    case "aws":
        instanceType := resource.GetTags()["instanceType"]
        price = p.awsPrices[instanceType]
        detail = "AWS on-demand"
    case "azure":
        vmSize := resource.GetTags()["vmSize"]
        price = p.azurePrices[vmSize]
        detail = "Azure Pay-As-You-Go"
    }

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        price,
        detail,
    ), nil
}
```

### Plugin with Custom Pricing Logic

```go
type CustomPricingPlugin struct {
    *pluginsdk.BasePlugin
    basePrices     map[string]float64
    discountTiers  map[string]float64
}

func NewCustomPricingPlugin() *CustomPricingPlugin {
    base := pluginsdk.NewBasePlugin("custom-pricing")
    base.Matcher().AddProvider("aws")

    return &CustomPricingPlugin{
        BasePlugin: base,
        basePrices: map[string]float64{
            "t3.micro":  0.0104,
            "t3.small":  0.0208,
            "t3.medium": 0.0416,
        },
        discountTiers: map[string]float64{
            "dev":        1.0,    // No discount
            "staging":    0.9,    // 10% discount
            "production": 0.8,    // 20% discount
        },
    }
}

func (p *CustomPricingPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()

    if !p.Matcher().Supports(resource) {
        return nil, pluginsdk.NotSupportedError(resource)
    }

    // Get base price
    instanceType := resource.GetTags()["instanceType"]
    basePrice := p.basePrices[instanceType]

    // Apply environment-based discount
    env := resource.GetTags()["environment"]
    discount := p.discountTiers[env]
    if discount == 0 {
        discount = 1.0
    }

    finalPrice := basePrice * discount

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        finalPrice,
        "custom pricing with discount",
    ), nil
}
```

---

## Related Documentation

- [Plugin Development Guide](plugin-development.md) - Building plugins
- [Plugin Examples](plugin-examples.md) - Common patterns
- [Plugin Protocol](../architecture/plugin-protocol.md) - gRPC spec
- [Plugin Checklist](plugin-checklist.md) - Implementation checklist
