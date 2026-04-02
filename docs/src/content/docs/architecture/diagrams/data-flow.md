---
layout: default
title: Data Flow Diagram
description: Sequence diagram showing how data flows from Pulumi through FinFocus to plugins and external APIs
---

This diagram shows the complete data flow from Pulumi plan generation through
cost calculation and output rendering.

```mermaid
sequenceDiagram
    actor User
    participant Pulumi as Pulumi CLI
    participant CLI as FinFocus CLI
    participant Engine as Engine
    participant Ingest as Ingest
    participant Registry as Registry
    participant PluginHost as Plugin Host
    participant Plugin as Cost Plugin
    participant API as External API

    User->>Pulumi: pulumi preview --json > plan.json
    Pulumi-->>User: plan.json

    User->>CLI: finfocus cost projected<br/>--pulumi-json plan.json
    activate CLI

    CLI->>Engine: CalculateProjectedCost(planPath)
    activate Engine

    Engine->>Ingest: ParsePulumiPlan(planPath)
    activate Ingest
    Ingest->>Ingest: Read JSON file
    Ingest->>Ingest: Extract resources
    Ingest->>Ingest: Build ResourceDescriptors
    Ingest-->>Engine: []ResourceDescriptor
    deactivate Ingest

    Engine->>Registry: DiscoverPlugins()
    activate Registry
    Registry->>Registry: Scan ~/.finfocus/plugins/
    Registry->>Registry: Validate manifests
    Registry-->>Engine: []PluginMetadata
    deactivate Registry

    Engine->>PluginHost: ConnectToPlugin(name, version)
    activate PluginHost
    PluginHost->>PluginHost: Launch plugin process
    PluginHost->>Plugin: gRPC Connect
    activate Plugin
    Plugin-->>PluginHost: Connection established
    PluginHost-->>Engine: Plugin client
    deactivate PluginHost

    loop For each resource
        Engine->>Plugin: Supports(ResourceDescriptor)
        Plugin-->>Engine: SupportsResponse{supported=true}

        Engine->>Plugin: GetProjectedCost(ResourceDescriptor)
        activate Plugin
        Plugin->>API: Query pricing data
        activate API
        API-->>Plugin: Pricing response
        deactivate API
        Plugin->>Plugin: Calculate cost
        Plugin-->>Engine: ProjectedCostResponse
        deactivate Plugin

        Engine->>Engine: Aggregate costs
    end

    Engine->>Engine: Format output (table/json/ndjson)
    Engine-->>CLI: FormattedOutput
    deactivate Engine

    CLI->>User: Display cost report
    deactivate CLI
```

## Data Flow Stages

### 1. Plan Generation

User generates Pulumi infrastructure plan using `pulumi preview --json`,
which outputs resource definitions to a JSON file.

### 2. CLI Invocation

User invokes FinFocus CLI with the plan file. The CLI parses command-line
arguments and routes the request to the appropriate Engine method.

### 3. Resource Ingestion

The Engine delegates to the Ingest component to parse the Pulumi JSON plan
and convert it into internal ResourceDescriptor objects containing provider,
type, SKU, region, and tags.

### 4. Plugin Discovery

The Engine queries the Registry to discover available cost source plugins by
scanning the plugin directory structure (`~/.finfocus/plugins/`).

### 5. Plugin Connection

The Engine uses the Plugin Host to establish gRPC connections to discovered
plugins. The Plugin Host launches plugin processes and manages their lifecycle.

### 6. Cost Calculation Loop

For each resource descriptor:

- Engine checks if plugin supports the resource type
- Engine requests cost calculation from plugin
- Plugin queries external API for pricing data
- Plugin performs cost calculations
- Plugin returns cost results to Engine
- Engine aggregates costs across all resources

### 7. Output Formatting

The Engine formats the aggregated cost data according to the user's requested
output format (table, JSON, or NDJSON).

### 8. Result Display

The CLI displays the formatted cost report to the user.

## Data Transformation Pipeline

```text
Pulumi Plan JSON
    ↓
Resource Descriptors (internal/engine types)
    ↓
Plugin gRPC Requests (protobuf)
    ↓
External API Requests (REST/gRPC)
    ↓
External API Responses
    ↓
Plugin gRPC Responses (protobuf)
    ↓
Aggregated Cost Results (internal/engine types)
    ↓
Formatted Output (table/JSON/NDJSON)
```

## Error Handling Flow

```mermaid
graph LR
    A[Resource] --> B{Plugin<br/>Supports?}
    B -->|Yes| C[Query Plugin]
    B -->|No| D[Try Next Plugin]
    C --> E{Success?}
    E -->|Yes| F[Use Result]
    E -->|No| G[Try Fallback Spec]
    D --> H{More<br/>Plugins?}
    H -->|Yes| A
    H -->|No| G
    G --> I{Spec<br/>Found?}
    I -->|Yes| F
    I -->|No| J[Return Placeholder]
```

## Fallback Strategy

When plugins are unavailable or don't support a resource type, the Engine
falls back to local YAML pricing specifications in `~/.finfocus/specs/`.
If no spec is found, a placeholder cost is returned to ensure the system
always produces output.

---

**Related Documentation:**

- [System Architecture](system-architecture.md) - Component overview
- [Plugin Protocol](../plugin-protocol.md) - gRPC protocol details
- [Cost Calculation Flow](cost-calculation-flow.md) - Detailed calculation
  logic
