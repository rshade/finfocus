---
layout: default
title: System Overview
description: High-level architecture and design of FinFocus cost calculation system
---

FinFocus is a CLI tool and plugin host system for calculating cloud
infrastructure costs from Pulumi infrastructure definitions. It provides
both projected cost estimates and actual historical cost analysis through
a plugin-based architecture.

## Architecture Diagram

See the [System Architecture Diagram](diagrams/system-architecture.md) for
a visual representation of all components and their relationships.

## Core Components

### CLI Layer

**Location:** `internal/cli`

The CLI layer provides a Cobra-based command-line interface with the
following subcommands:

- `cost projected` - Calculate projected costs from Pulumi preview JSON
- `cost actual` - Fetch actual historical costs with time ranges
- `plugin list` - List installed plugins
- `plugin validate` - Validate plugin installations

**Design Pattern:** Command pattern with Cobra framework

### Engine

**Location:** `internal/engine`

The Engine is the core orchestration layer that coordinates between plugins,
local specifications, and output formatting.

**Responsibilities:**

- Cost calculation orchestration
- Plugin selection and fallback logic
- Resource mapping and aggregation
- Multiple output format support (table, JSON, NDJSON)
- Actual cost pipeline with advanced querying

**Key Methods:**

- `CalculateProjectedCost()` - Estimate future costs from Pulumi plans
- `GetActualCostWithOptions()` - Query historical costs with filtering
- `CreateCrossProviderAggregation()` - Aggregate costs across providers

### Ingest

**Location:** `internal/ingest`

The Ingest component parses Pulumi JSON output and converts it to internal
resource descriptors.

**Process:**

1. Read `pulumi preview --json` output file
2. Extract resource definitions
3. Parse provider, type, SKU, region, tags
4. Build ResourceDescriptor objects

**Output:** Array of ResourceDescriptor objects for Engine processing

### Plugin Host

**Location:** `internal/pluginhost`

The Plugin Host manages gRPC connections to external cost source plugins.

**Components:**

- `Client` - Wraps plugin gRPC connections
- `ProcessLauncher` - Launches plugins as TCP processes
- `StdioLauncher` - Alternative stdio-based communication

**Lifecycle Management:**

- Process spawning and monitoring
- Connection establishment with retries
- Graceful shutdown handling

See [Plugin Lifecycle Diagram](diagrams/plugin-lifecycle.md) for detailed
state transitions.

### Registry

**Location:** `internal/registry`

The Registry discovers and manages plugin lifecycle from the filesystem.

**Discovery Process:**

1. Scan `~/.finfocus/plugins/<name>/<version>/` directories
2. Validate plugin binaries (Unix permissions or Windows .exe extension)
3. Load optional `plugin.manifest.json` metadata
4. Build available plugins catalog

**Directory Structure:**

```text
~/.finfocus/
└── plugins/
    ├── kubecost/
    │   └── 1.0.0/
    │       ├── kubecost-plugin
    │       └── plugin.manifest.json
    └── vantage/
        └── 1.0.0/
            ├── vantage-plugin
            └── plugin.manifest.json
```

### Spec System

**Location:** `internal/spec`

The Spec System provides fallback pricing when plugins are unavailable.

**Specification Format:**

```yaml
provider: aws
resource_type: ec2
sku: t3.micro
region: us-east-1
billing_mode: per_hour
rate_per_unit: 0.0104
currency: USD
description: AWS EC2 t3.micro instance pricing
```

**Location:** `~/.finfocus/specs/`

## Data Flow

See the [Data Flow Diagram](diagrams/data-flow.md) for a complete sequence
diagram showing how data flows through the system.

### High-Level Flow

```text
Pulumi JSON → Resource Descriptors → Plugin Queries →
Cost Results → Aggregation → Output Rendering
```

### Projected Cost Flow

1. User generates Pulumi plan with `pulumi preview --json`
2. User runs `finfocus cost projected --pulumi-json plan.json`
3. CLI passes plan path to Engine
4. Engine delegates to Ingest to parse JSON
5. Ingest extracts resources and builds ResourceDescriptors
6. Engine discovers plugins via Registry
7. Engine connects to plugins via PluginHost
8. For each resource, Engine queries plugin for projected cost
9. Plugin queries external API for pricing data
10. Plugin calculates monthly cost estimate
11. Engine aggregates all costs
12. Engine formats output (table/JSON/NDJSON)
13. CLI displays result to user

### Actual Cost Flow

1. User runs `finfocus cost actual --start-date X --end-date Y`
2. CLI builds ActualCostRequest with time range and filters
3. Engine connects to plugins
4. For each resource, Engine queries plugin for actual costs
5. Plugin queries external API for historical cost data
6. Plugin returns daily/monthly cost breakdowns
7. Engine aggregates costs with grouping (resource, type, provider, date)
8. Engine validates currency consistency
9. Engine formats output
10. CLI displays result to user

## Key Design Patterns

### Plugin Architecture

**Pattern:** Plugin-based extensibility

**Benefits:**

- Third-party cost source integration without core changes
- Language-agnostic plugin development (any language with gRPC)
- Isolated plugin failures don't crash main system
- Independent plugin versioning and deployment

**Protocol:** gRPC using protocol buffers from `finfocus-spec` repository

See [Plugin Protocol](plugin-protocol.md) for complete gRPC specification.

### Fallback Pattern

**Pattern:** Try plugins first, fall back to local specifications

**Flow:**

```text
1. Try available plugins for resource type
2. If plugin fails or doesn't support:
   → Load local YAML spec from ~/.finfocus/specs/
3. If spec not found:
   → Return placeholder cost ($0.00 with "unknown" source)
```

**Guarantee:** System always produces output, even with incomplete data

### Registry Pattern

**Pattern:** Dynamic plugin discovery from filesystem

**Benefits:**

- No hardcoded plugin list in core
- Users install plugins by copying to directory
- Multiple versions can coexist
- Platform-specific binary detection

### Adapter Pattern

**Pattern:** Engine adapts between CLI expectations and plugin realities

**Implementation:**

- `internal/proto/adapter.go` - Bridges engine types with proto types
- Converts between internal ResourceDescriptor and proto ResourceDescriptor
- Handles error translation from proto to Go errors

## Cost Calculation

See [Cost Calculation](cost-calculation.md) for detailed algorithms and
[Cost Calculation Flow Diagram](diagrams/cost-calculation-flow.md) for
flowchart.

### Projected Cost Formula

```text
Monthly Cost = Unit Price × Hours Per Month

Where:
  Hours Per Month = 730 (standard constant)
  Unit Price = From plugin or local spec
```

### Actual Cost Aggregation

```text
Total Cost = Σ (Daily Cost) for date in [start, end]

With optional grouping by:
  - Resource ID
  - Resource Type (ec2, s3, etc.)
  - Provider (aws, azure, gcp)
  - Date (daily or monthly aggregation)
```

### Cross-Provider Aggregation

**Feature:** Aggregate costs across multiple cloud providers

**Validation:**

- Ensures consistent currency (USD, EUR, etc.)
- Validates date ranges (end > start)
- Supports time-based grouping only (daily, monthly)

**Error Handling:**

- `ErrMixedCurrencies` - Different currencies detected
- `ErrInvalidGroupBy` - Non-time-based grouping attempted
- `ErrInvalidDateRange` - Invalid date range provided

## Error Handling Strategy

### Transient Errors

**Examples:** Network timeout, rate limiting, service unavailable

**Handling:**

- Retry up to 3 times with exponential backoff
- Wait intervals: 100ms, 200ms, 400ms
- Continue with next plugin or fallback to specs

### Permanent Errors

**Examples:** Resource not found, invalid credentials, unsupported region

**Handling:**

- No retry
- Immediately fall back to local specs
- Log error for user visibility

### Configuration Errors

**Examples:** Missing API key, invalid endpoint, missing plugin binary

**Handling:**

- Report clear error message to user
- Suggest configuration fixes
- Exit with non-zero status code

## Output Formats

### Table Format

```text
┌──────────────────┬─────────┬──────────┬─────────────┐
│ Resource         │ Type    │ Cost/mo  │ Source      │
├──────────────────┼─────────┼──────────┼─────────────┤
│ aws-ec2-i-123    │ t3.micro│   $7.59  │ vantage     │
│ aws-ec2-i-456    │ t3.micro│   $7.59  │ vantage     │
├──────────────────┼─────────┼──────────┼─────────────┤
│ Total            │         │  $15.18  │             │
└──────────────────┴─────────┴──────────┴─────────────┘
```

### JSON Format

```json
{
  "resources": [
    {
      "id": "aws-ec2-i-123",
      "type": "t3.micro",
      "cost_per_month": 7.59,
      "currency": "USD",
      "source": "vantage"
    }
  ],
  "total_cost": 15.18,
  "currency": "USD"
}
```

### NDJSON Format

```json
{"id":"aws-ec2-i-123","type":"t3.micro","cost_per_month":7.59}
{"id":"aws-ec2-i-456","type":"t3.micro","cost_per_month":7.59}
```

## Dependencies

### External Libraries

- `github.com/spf13/cobra` - CLI framework
- `google.golang.org/grpc` - Plugin communication
- `gopkg.in/yaml.v3` - YAML spec parsing
- `github.com/rshade/finfocus-spec` - Protocol definitions

### Protocol Version

Current protocol version: v0.1.0 (frozen and integrated)

**Protocol Repository:**
`github.com/rshade/finfocus-spec/proto/finfocus/v1/costsource.proto`

## Plugin Protocol Integration

The project uses real protocol buffer definitions from the `finfocus-spec`
repository.

**Integration:**

- `internal/proto/adapter.go` - Adapts between engine and proto types
- Generated SDK: `github.com/rshade/finfocus-spec/sdk/go/proto`
- gRPC v1.74.2, protobuf v1.36.7

**Services:**

- `CostSourceService` - Core cost operations
- `ObservabilityService` - Health checks and metrics (future)

See [Plugin Protocol](plugin-protocol.md) for complete gRPC specification.

## Integration Example

See [Integration Example Diagram](diagrams/integration-example.md) for a
complete end-to-end example showing Pulumi → FinFocus → Vantage API
integration.

---

**Related Documentation:**

- [Plugin Protocol](plugin-protocol.md) - gRPC protocol specification
- [Cost Calculation](cost-calculation.md) - Detailed calculation algorithms
- [CLI Commands](../reference/cli-commands.md) - Command-line reference
- [API Reference](../reference/api-reference.md) - Complete API docs
