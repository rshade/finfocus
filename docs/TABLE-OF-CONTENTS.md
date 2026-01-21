---
layout: default
title: Table of Contents
description: Complete navigation guide for FinFocus documentation
---

Welcome to FinFocus documentation. Use this page to navigate all available resources.

## Quick Navigation by Audience

### 👤 For End Users

1. Start: [5-Minute Quickstart](getting-started/quickstart.md)
2. Install: [Installation Guide](getting-started/installation.md)
3. Learn: [User Guide](guides/user-guide.md)
4. Configure: [Budgets](guides/budgets.md) | [Recommendations](guides/recommendations.md)
5. Command Reference: [CLI Commands](reference/cli-commands.md)
6. Get Help: [FAQ](support/faq.md) | [Troubleshooting](support/troubleshooting.md)

### 🛠️ For Engineers/Plugin Developers

1. Start: [Developer Guide](guides/developer-guide.md)
2. Build: [Plugin Development](plugins/plugin-development.md)
3. Reference: [Plugin SDK](plugins/plugin-sdk.md)
4. Example: [Vantage Plugin](plugins/vantage/README.md)
5. Contribute: [Contributing Guide](support/contributing.md)

### 🏗️ For Software Architects

1. Overview: [Architecture Guide](guides/architect-guide.md)
2. Design: [System Architecture](architecture/system-overview.md)
3. Patterns: [Plugin Protocol](architecture/plugin-protocol.md)
4. Deploy: [Deployment Guide](deployment/deployment.md)
5. Secure: [Security Guide](deployment/security.md)

### 💼 For Executives/Product

1. Value: [Business Value & ROI](guides/business-value.md)
2. See Demo: [Quickstart](getting-started/quickstart.md)
3. Timeline: [Roadmap](architecture/roadmap.md)
4. Details: [System Architecture](guides/architect-guide.md)

---

## Full Documentation Index

### 📘 Getting Started

| Page                                                       | Purpose                   | Audience       |
| ---------------------------------------------------------- | ------------------------- | -------------- |
| [5-Minute Quickstart](getting-started/quickstart.md)       | Get running in 5 minutes  | Everyone       |
| [Installation Guide](getting-started/installation.md)      | Step-by-step installation | End Users, Ops |
| [Prerequisites](getting-started/prerequisites.md)          | System requirements       | Everyone       |
| [Examples](getting-started/examples/)                      | Practical usage examples  | End Users      |
| [Vantage Setup](getting-started/examples/vantage-setup.md) | Vantage plugin example    | End Users      |
| [Local Pricing](getting-started/examples/local-pricing.md) | Local specs example       | End Users      |
| [Multi-Plugin](getting-started/examples/multi-plugin.md)   | Multiple plugins example  | Advanced Users |

### 📚 Comprehensive Guides (by Audience)

| Page                                            | Purpose                  | Length |
| ----------------------------------------------- | ------------------------ | ------ |
| [User Guide](guides/user-guide.md)              | Complete end user guide  | Long   |
| [Developer Guide](guides/developer-guide.md)    | Complete engineer guide  | Long   |
| [Architecture Guide](guides/architect-guide.md) | Complete architect guide | Long   |
| [Business Value](guides/business-value.md)      | Executive summary & ROI  | Medium |

### 🚀 Feature Guides

| Page                                          | Purpose                                  |
| --------------------------------------------- | ---------------------------------------- |
| [Budget Configuration](guides/budgets.md)     | Configure budgets, alerts, and thresholds |
| [Recommendations](guides/recommendations.md)  | Use cost optimization recommendations    |
| [Accessibility](guides/accessibility.md)      | Configure colors, contrast, and TUI      |

### 🏗️ Architecture & Design

| Page                                                       | Purpose                                      |
| ---------------------------------------------------------- | -------------------------------------------- |
| [System Overview](architecture/system-overview.md)         | High-level architecture with diagrams        |
| [Core Concepts](architecture/core-concepts.md)             | Key concepts (resources, costs, aggregation) |
| [Plugin Protocol](architecture/plugin-protocol.md)         | gRPC plugin specification                    |
| [Cost Calculation](architecture/cost-calculation.md)       | How costs are calculated                     |
| [Actual vs Projected](architecture/actual-vs-projected.md) | Understanding cost types                     |
| [Roadmap](architecture/roadmap.md)                         | Planned features and timeline                |

### 📊 Architecture Diagrams

| Diagram                                                                 | Shows                                   |
| ----------------------------------------------------------------------- | --------------------------------------- |
| [System Architecture](architecture/diagrams/system-architecture.md)     | Component overview                      |
| [Data Flow](architecture/diagrams/data-flow.md)                         | How data flows through system           |
| [Plugin Lifecycle](architecture/diagrams/plugin-lifecycle.md)           | Plugin discovery & startup              |
| [Cost Calculation Flow](architecture/diagrams/cost-calculation-flow.md) | Cost calculation pipeline               |
| [Integration Example](architecture/diagrams/integration-example.md)     | End-to-end: Pulumi → FinFocus → Vantage |

### 🔌 Plugin Documentation

#### Development

| Page                                                      | Purpose                    |
| --------------------------------------------------------- | -------------------------- |
| [Plugin Development Guide](plugins/plugin-development.md) | How to build a plugin      |
| [Plugin SDK Reference](plugins/plugin-sdk.md)             | API and SDK documentation  |
| [Plugin Examples](plugins/plugin-examples.md)             | Code examples and patterns |
| [Plugin Checklist](plugins/plugin-checklist.md)           | Ensure plugin is complete  |

#### Vantage (IN PROGRESS)

| Page                                                  | Purpose                        |
| ----------------------------------------------------- | ------------------------------ |
| [Vantage README](plugins/vantage/README.md)           | Overview and status            |
| [Setup Guide](plugins/vantage/setup.md)               | Installation and configuration |
| [Authentication](plugins/vantage/authentication.md)   | API key management             |
| [Features](plugins/vantage/features.md)               | Supported features             |
| [Cost Mapping](plugins/vantage/cost-mapping.md)       | How costs map to FinFocus      |
| [Troubleshooting](plugins/vantage/troubleshooting.md) | Common issues                  |

#### Kubecost (PLANNED)

| Page                                           | Purpose                     |
| ---------------------------------------------- | --------------------------- |
| [Coming Soon](plugins/kubecost/coming-soon.md) | Timeline and features       |
| [Differences](plugins/kubecost/differences.md) | How it differs from Vantage |

#### Future Plugins

| Page                                                            | Status                   |
| --------------------------------------------------------------- | ------------------------ |
| [Flexera Coming Soon](plugins/flexera/coming-soon.md)           | FUTURE (customer demand) |
| [Cloudability Coming Soon](plugins/cloudability/coming-soon.md) | FUTURE (customer demand) |

### 📖 Reference Documentation

| Page                                                        | Purpose                      |
| ----------------------------------------------------------- | ---------------------------- |
| [CLI Commands](reference/cli-commands.md)                   | Complete command reference   |
| [CLI Flags](reference/cli-flags.md)                         | Detailed flag documentation  |
| [Configuration](reference/config-reference.md)              | Configuration file reference |
| [API Reference](reference/api-reference.md)                 | gRPC API documentation       |
| [Error Codes](reference/error-codes.md)                     | Error codes and solutions    |
| [Environment Variables](reference/environment-variables.md) | Env var reference            |

### 🚀 Deployment & Operations

| Page                                                             | Purpose                 |
| ---------------------------------------------------------------- | ----------------------- |
| [Installation Guide](deployment/installation.md)                 | Detailed installation   |
| [Configuration Guide](deployment/configuration.md)               | Configuration how-to    |
| [Deployment Architecture](deployment/deployment-architecture.md) | Deployment diagram      |
| [Docker](deployment/docker.md)                                   | Docker deployment       |
| [Kubernetes](deployment/kubernetes.md)                           | K8s deployment (future) |
| [CI/CD Integration](deployment/cicd-integration.md)              | Pipeline integration    |
| [Security](deployment/security.md)                               | Security best practices |
| [Troubleshooting](deployment/troubleshooting.md)                 | Operational issues      |

### 💬 Support & Community

| Page                                            | Purpose                    |
| ----------------------------------------------- | -------------------------- |
| [FAQ](support/faq.md)                           | Frequently asked questions |
| [Troubleshooting](support/troubleshooting.md)   | Problem-solving by symptom |
| [Contributing](support/contributing.md)         | How to contribute          |
| [Code of Conduct](support/code-of-conduct.md)   | Community guidelines       |
| [Support Channels](support/support-channels.md) | Where to get help          |

### 📋 Setup & Configuration

| Page                                        | Purpose                       |
| ------------------------------------------- | ----------------------------- |
| [GitHub Pages Setup](GITHUB-PAGES-SETUP.md) | Enable docs deployment        |
| [Developer Setup](GETTING-STARTED-DEV.md)   | Local development environment |

### 📚 Meta Documentation

| Page                          | Purpose                              |
| ----------------------------- | ------------------------------------ |
| [Documentation Plan](plan.md) | Documentation architecture strategy  |
| [LLM Index](llms.txt)         | Machine-readable documentation index |

---

## Documentation by Topic

### Getting Started (New Users)

1. [README](README.md) - Start here
2. [Quickstart](getting-started/quickstart.md) - 5 minutes
3. [Installation](getting-started/installation.md) - Install
4. [User Guide](guides/user-guide.md) - Learn

### Installation & Setup

- [Prerequisites](getting-started/prerequisites.md)
- [Installation Guide](getting-started/installation.md)
- [GitHub Pages Setup](GITHUB-PAGES-SETUP.md)
- [Developer Setup](GETTING-STARTED-DEV.md)

### Using FinFocus

- [Quickstart](getting-started/quickstart.md)
- [User Guide](guides/user-guide.md)
- [Budgets & Alerts](guides/budgets.md)
- [Cost Recommendations](guides/recommendations.md)
- [Accessibility Options](guides/accessibility.md)
- [CLI Commands](reference/cli-commands.md)
- [Examples](getting-started/examples/)

### Building Plugins

- [Developer Guide](guides/developer-guide.md)
- [Plugin Development](plugins/plugin-development.md)
- [Plugin SDK](plugins/plugin-sdk.md)
- [Vantage Example](plugins/vantage/README.md)
- [Plugin Examples](plugins/plugin-examples.md)

### System Design

- [Architecture Guide](guides/architect-guide.md)
- [System Overview](architecture/system-overview.md)
- [Plugin Protocol](architecture/plugin-protocol.md)
- [Cost Calculation](architecture/cost-calculation.md)
- [Architecture Diagrams](architecture/diagrams/)

### Operations & Deployment

- [Deployment Guide](deployment/deployment.md)
- [Deployment Architecture](deployment/deployment-architecture.md)
- [Configuration](deployment/configuration.md)
- [Docker](deployment/docker.md)
- [Security](deployment/security.md)
- [CI/CD Integration](deployment/cicd-integration.md)

### Troubleshooting & Support

- [FAQ](support/faq.md)
- [Troubleshooting](support/troubleshooting.md)
- [Error Codes](reference/error-codes.md)
- [Support Channels](support/support-channels.md)

### Contributing

- [Contributing Guide](support/contributing.md)
- [Code of Conduct](support/code-of-conduct.md)
- [Developer Setup](GETTING-STARTED-DEV.md)

### Business/Strategy

- [Business Value](guides/business-value.md)
- [Roadmap](architecture/roadmap.md)
- [Comparison](guides/comparison.md)

---

## Search Tips

Use Ctrl+F to search within pages:

- **"How do I..."** - See [User Guide](guides/user-guide.md)
- **"Setup"** - See [Installation Guide](getting-started/installation.md)
- **"Plugin"** - See [Plugin Documentation](plugins/)
- **"Error"** - See [Troubleshooting](support/troubleshooting.md)
- **"Architecture"** - See [Architecture Guide](guides/architect-guide.md)
- **"Cost"** - See [Cost Calculation](architecture/cost-calculation.md)

---

## Documentation Status

| Section         | Status      | Coverage |
| :-------------- | :---------- | :------- |
| Getting Started | ✅ Complete | 100%     |
| Guides          | ✅ Complete | 100%     |
| Architecture    | 🟡 Partial  | 60%      |
| Plugins         | 🟡 Partial  | 50%      |
| Reference       | 🟡 Partial  | 60%      |
| Deployment      | 🟡 Partial  | 50%      |
| Support         | ✅ Complete | 80%      |

---

## Quick Links

- [Home](README.md)
- [Quickstart](getting-started/quickstart.md)
- [User Guide](guides/user-guide.md)
- [CLI Reference](reference/cli-commands.md)
- [GitHub](https://github.com/rshade/finfocus)
- [Issues](https://github.com/rshade/finfocus/issues)

---

**Last Updated:** 2025-10-29

**Next Steps:** Choose your starting point above based on your role!
