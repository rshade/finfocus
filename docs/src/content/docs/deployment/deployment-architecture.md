---
title: Deployment Architecture Diagram
description: Reference deployment architecture for FinFocus environments
layout: default
---

This diagram summarizes how FinFocus is deployed across workstations, CI/CD, and containers.

```mermaid
graph TD
    Developer[Developer Workstation] --> CLI[FinFocus CLI]
    CLI --> Config[~/.finfocus/config.yaml]
    CLI --> Plugins[~/.finfocus/plugins]
    CLI --> Specs[~/.finfocus/specs]

    subgraph CI/CD Pipeline
        Runner[CI Runner] --> CLI
        Runner --> Cache[Plugin + Spec Cache]
        Runner --> Secrets[Pipeline Secrets]
    end

    subgraph Container Runtime
        Image[GHCR Image] --> CLI
        Volumes[Mounted Volumes] --> Config
        Volumes --> Plugins
        Volumes --> Specs
    end

    CLI --> Pulumi[Pulumi Plan JSON]
    CLI --> Reports[Cost Reports]

    Plugins --> ProviderAPIs[Cost Provider APIs]
    Specs --> LocalPricing[Local Pricing Specs]

    classDef core fill:#4A90E2,stroke:#2E5C8A,color:#fff
    classDef storage fill:#F5A623,stroke:#C77F1B,color:#fff
    classDef external fill:#BD10E0,stroke:#8B0AA8,color:#fff

    class CLI core
    class Config,Plugins,Specs,Cache,Volumes storage
    class Pulumi,ProviderAPIs,LocalPricing,Reports external
```

## How to Read the Diagram

- **CLI** executes cost analysis regardless of environment.
- **Config, plugins, and specs** live in the same directories whether local,
  cached in CI, or mounted into containers.
- **Pulumi plan JSON** remains the input to projected cost analysis.
- **Cost provider APIs** are accessed through plugins when configured.

## Related Guides

- [Deployment Overview](deployment.html)
- [Configuration Guide](configuration.html)
- [Docker Deployment](docker.html)
