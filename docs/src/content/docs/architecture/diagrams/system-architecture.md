---
layout: default
title: System Architecture Diagram
description: High-level component architecture of FinFocus showing all major components and their relationships
---

This diagram shows the high-level architecture of FinFocus, including all
major components and their relationships.

```mermaid
graph TB
    subgraph "User Interface"
        CLI[CLI Layer<br/>internal/cli]
    end

    subgraph "Core System"
        Engine[Engine<br/>internal/engine<br/>Cost Orchestration]
        Ingest[Ingest<br/>internal/ingest<br/>Pulumi JSON Parser]
        Registry[Registry<br/>internal/registry<br/>Plugin Discovery]
    end

    subgraph "Plugin System"
        PluginHost[Plugin Host<br/>internal/pluginhost<br/>gRPC Client Manager]
        Plugin1[Plugin: Kubecost]
        Plugin2[Plugin: Vantage]
        Plugin3[Plugin: Custom]
    end

    subgraph "Fallback System"
        SpecLoader[Spec Loader<br/>internal/spec<br/>YAML Parser]
        SpecFiles[(Local Pricing Specs<br/>~/.finfocus/specs/)]
    end

    subgraph "External Systems"
        PulumiCLI[Pulumi CLI<br/>pulumi preview --json]
        KubecostAPI[Kubecost API]
        VantageAPI[Vantage API]
        CloudAPIs[Cloud Provider APIs]
    end

    subgraph "Plugin Storage"
        PluginDir[(Plugin Binaries<br/>~/.finfocus/plugins/)]
    end

    CLI --> Engine
    Engine --> Ingest
    Engine --> PluginHost
    Engine --> SpecLoader

    Ingest --> PulumiCLI

    PluginHost --> Registry
    Registry --> PluginDir
    PluginHost --> Plugin1
    PluginHost --> Plugin2
    PluginHost --> Plugin3

    Plugin1 -.gRPC.-> KubecostAPI
    Plugin2 -.gRPC.-> VantageAPI
    Plugin3 -.gRPC.-> CloudAPIs

    SpecLoader --> SpecFiles

    classDef coreComponent fill:#4A90E2,stroke:#2E5C8A,color:#fff
    classDef pluginComponent fill:#7ED321,stroke:#5A9E19,color:#fff
    classDef storage fill:#F5A623,stroke:#C77F1B,color:#fff
    classDef external fill:#BD10E0,stroke:#8B0AA8,color:#fff

    class CLI,Engine,Ingest,Registry coreComponent
    class PluginHost,Plugin1,Plugin2,Plugin3 pluginComponent
    class SpecLoader,SpecFiles,PluginDir storage
    class PulumiCLI,KubecostAPI,VantageAPI,CloudAPIs external
```

## Component Overview

### User Interface Layer

- **CLI Layer** (`internal/cli`) - Cobra-based command-line interface
  providing subcommands for cost operations and plugin management

### Core System Components

- **Engine** (`internal/engine`) - Orchestrates cost calculations, coordinates
  between plugins and specs, handles output formatting
- **Ingest** (`internal/ingest`) - Parses Pulumi JSON output and converts to
  resource descriptors
- **Registry** (`internal/registry`) - Discovers and manages plugin lifecycle
  from filesystem

### Plugin System

- **Plugin Host** (`internal/pluginhost`) - Manages gRPC connections to
  plugins, handles process lifecycle
- **Plugins** - External cost source integrations (Kubecost, Vantage, custom
  implementations)

### Fallback System

- **Spec Loader** (`internal/spec`) - Loads local YAML pricing specifications
  when plugins unavailable
- **Spec Files** - Local YAML-based pricing data stored in user's home
  directory

### External Systems

- **Pulumi CLI** - Generates infrastructure plan JSON via
  `pulumi preview --json`
- **Cost Source APIs** - External APIs (Kubecost, Vantage, cloud providers)
  queried by plugins

## Communication Patterns

- **Solid lines** - Direct function calls and synchronous communication
- **Dotted lines** - gRPC network communication with external APIs
- **Storage cylinders** - Filesystem-based data sources

## Design Patterns

- **Plugin Architecture** - Extensible via external gRPC plugins
- **Fallback Pattern** - Local specs used when plugins unavailable
- **Registry Pattern** - Dynamic plugin discovery from filesystem
- **Adapter Pattern** - Engine adapts between CLI, plugins, and specs

---

**Related Documentation:**

- [System Overview](../system-overview.md) - Detailed architecture explanation
- [Plugin Protocol](../plugin-protocol.md) - gRPC protocol specification
- [Data Flow](data-flow.md) - How data flows through the system
