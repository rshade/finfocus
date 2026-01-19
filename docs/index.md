---
layout: default
title: FinFocus Documentation
description: Cost visibility for Pulumi infrastructure. Calculate projected and actual costs.
---

Welcome to the FinFocus documentation hub. Whether you're a user, engineer, architect, or
business stakeholder, you'll find comprehensive guides to help you succeed with FinFocus.

## Quick Navigation

### I want to... (pick your path)

<table>
  <tr>
    <th>👤 End User</th>
    <th>🛠️ Engineer/Developer</th>
    <th>🏗️ Software Architect</th>
    <th>💼 Business/CEO</th>
  </tr>
  <tr>
    <td>
      <strong>See costs in 5 minutes</strong><br>
      <a href="getting-started/quickstart.html">Quickstart</a><br>
      <a href="getting-started/installation.html">Install</a><br>
      <a href="reference/cli-commands.html">CLI Commands</a><br>
      <a href="support/faq.html">FAQ</a>
    </td>
    <td>
      <strong>Build a plugin</strong><br>
      <a href="guides/developer-guide.html">Developer Guide</a><br>
      <a href="plugins/plugin-development.html">Plugin Dev</a><br>
      <a href="plugins/plugin-sdk.html">SDK Reference</a><br>
      <a href="support/contributing.html">Contributing</a>
    </td>
    <td>
      <strong>Integrate with our system</strong><br>
      <a href="guides/architect-guide.html">Architect Guide</a><br>
      <a href="architecture/system-overview.html">Architecture</a><br>
      <a href="deployment/deployment.html">Deployment</a><br>
      <a href="deployment/security.html">Security</a>
    </td>
    <td>
      <strong>Understand the value</strong><br>
      <a href="guides/business-value.html">Value Prop</a><br>
      <a href="guides/comparison.html">Competitive</a><br>
      <a href="architecture/roadmap.html">Roadmap</a><br>
      <a href="getting-started/quickstart.html">Demo (5m)</a>
    </td>
  </tr>
</table>

---

## Documentation Overview

### 📚 Comprehensive Guides

- **[User Guide](guides/user-guide.html)** - Complete guide for end users
- **[Developer Guide](guides/developer-guide.html)** - Complete guide for plugin developers
- **[Architect Guide](guides/architect-guide.html)** - Complete guide for software architects
- **[Business Value](guides/business-value.html)** - Value proposition and ROI

### 🚀 Getting Started (Recommended for first-time users)

- **[5-Minute Quickstart](getting-started/quickstart.html)** - Get costs in 5 minutes
- **[Installation Guide](getting-started/installation.html)** - Step-by-step installation
- **[Prerequisites](getting-started/prerequisites.html)** - System requirements
- **[Examples](getting-started/examples/)** - Practical examples with Vantage, local pricing, and multiple plugins

### 🏗️ Architecture & Design

- **[System Overview](architecture/system-overview.html)** - High-level architecture with diagrams
- **[Core Concepts](architecture/core-concepts.html)** - Key concepts explained
- **[Plugin Protocol](architecture/plugin-protocol.html)** - gRPC plugin protocol specification
- **[Cost Calculation](architecture/cost-calculation.html)** - How costs are calculated and aggregated
- **[Roadmap](architecture/roadmap.html)** - Planned features and timeline

### 🔌 Plugin Documentation

#### For Plugin Developers

- **[Plugin Development Guide](plugins/plugin-development.html)** - How to build a FinFocus plugin
- **[Plugin SDK Reference](plugins/plugin-sdk.html)** - API and SDK documentation
- **[Plugin Examples](plugins/plugin-examples.html)** - Code patterns and examples
- **[Plugin Checklist](plugins/plugin-checklist.html)** - Ensure your plugin is complete

#### Vantage Plugin (IN PROGRESS)

- **[Setup Guide](plugins/vantage/setup.html)** - Get started with Vantage
- **[Authentication](plugins/vantage/authentication.html)** - API key management
- **[Features](plugins/vantage/features.html)** - What's supported
- **[Troubleshooting](plugins/vantage/troubleshooting.html)** - Common issues

#### Future Plugins (PLANNED)

- **[Kubecost](plugins/kubecost/coming-soon.html)** - Kubernetes cost allocation
- **[Flexera](plugins/flexera/coming-soon.html)** - Multi-cloud cost management
- **[Cloudability](plugins/cloudability/coming-soon.html)** - Enterprise cost visibility

### 📖 Reference Documentation

- **[CLI Commands](reference/cli-commands.html)** - Complete command reference
- **[Configuration](reference/config-reference.html)** - Configuration options
- **[API Reference](reference/api-reference.html)** - gRPC API documentation
- **[Error Codes](reference/error-codes.html)** - Error codes and solutions

### 🚢 Deployment & Operations

- **[Installation Guide](deployment/installation.html)** - Detailed installation procedures
- **[Configuration Guide](deployment/configuration.html)** - How to configure
- **[Deployment Architecture](deployment/deployment-architecture.html)** - Deployment diagram
- **[Docker](deployment/docker.html)** - Docker deployment
- **[CI/CD Integration](deployment/cicd-integration.html)** - Pipeline integration
- **[Security](deployment/security.html)** - Security best practices
- **[Troubleshooting](deployment/troubleshooting.html)** - Operational issues

### 💬 Support & Community

- **[FAQ](support/faq.html)** - Frequently asked questions
- **[Troubleshooting Guide](support/troubleshooting.html)** - Problem-solving by symptom
- **[Contributing](support/contributing.html)** - How to contribute to FinFocus
- **[Code of Conduct](support/code-of-conduct.html)** - Community guidelines
- **[Support Channels](support/support-channels.html)** - Where to get help

---

## Key Concepts

### What is FinFocus?

FinFocus is a CLI tool that calculates cloud infrastructure costs from Pulumi infrastructure definitions. It provides:

- **Projected Costs** - Estimated costs from your infrastructure code
- **Actual Costs** - Real costs from cloud provider APIs
- **Cost Changes** - Understand what changed and by how much

### How It Works

```text
1. You define infrastructure with Pulumi
2. FinFocus reads your Pulumi definitions
3. Plugins fetch pricing and cost data
4. FinFocus calculates and displays results
```

### Plugin-Based Architecture

FinFocus uses a plugin system to support multiple cost providers:

- **Vantage** (IN PROGRESS) - Multi-cloud cost aggregation
- **Kubecost** (PLANNED) - Kubernetes cost allocation
- **Flexera** (FUTURE) - Enterprise cost management
- **Cloudability** (FUTURE) - Cloud cost visibility
- **Local Specs** (ALWAYS) - No external service needed

---

## Documentation Structure

See [plan.html](plan.html) for complete documentation architecture, maintenance strategy, and implementation timeline.

---

## Finding What You Need

### By Role

- **DevOps/Infrastructure Engineer** → Start with [Installation](getting-started/installation.html) → [Quickstart](getting-started/quickstart.html)
- **FinOps Engineer** → Start with [Business Value](guides/business-value.html) → [Architecture](architecture/system-overview.html)
- **Plugin Developer** → Start with [Developer Guide](guides/developer-guide.html) → [Plugin Development](plugins/plugin-development.html)
- **Platform Architect** → Start with [Architect Guide](guides/architect-guide.html) → [Architecture](architecture/system-overview.html)
- **Executive/CEO** → Start with [Business Value](guides/business-value.html) → [Roadmap](architecture/roadmap.html)

### By Use Case

- **I just installed FinFocus** → [Quickstart](getting-started/quickstart.html)
- **I want to integrate with Vantage** → [Vantage Setup](plugins/vantage/setup.html)
- **I'm building a custom plugin** → [Plugin Development](plugins/plugin-development.html)
- **I'm integrating with CI/CD** → [CI/CD Integration](deployment/cicd-integration.html)
- **Something's not working** → [Troubleshooting](support/troubleshooting.html)

### By Problem

- **"How do I install FinFocus?"** → [Installation Guide](getting-started/installation.html)
- **"How do I configure it?"** → [Configuration Guide](deployment/configuration.html)
- **"How do I build a plugin?"** → [Plugin Development](plugins/plugin-development.html)
- **"What's the cost calculation?"** → [Cost Calculation](architecture/cost-calculation.html)
- **"Where's the CLI reference?"** → [CLI Commands](reference/cli-commands.html)

---

## Site Map

```text
📄 Getting Started
  ├─ Quickstart (5 minutes)
  ├─ Installation
  ├─ Prerequisites
  └─ Examples
      ├─ Vantage Setup
      ├─ Local Pricing
      └─ Multi-Plugin

📘 Guides (by audience)
  ├─ User Guide (what it does)
  ├─ Developer Guide (how to extend)
  ├─ Architect Guide (how it's designed)
  └─ Business Value (why it matters)

🏗️ Architecture
  ├─ System Overview
  ├─ Core Concepts
  ├─ Plugin Protocol
  ├─ Cost Calculation
  ├─ Roadmap
  └─ Diagrams (visual)

🔌 Plugins
  ├─ Plugin Development
  ├─ SDK Reference
  ├─ Examples
  ├─ Checklist
  └─ Per-Plugin Documentation
      ├─ Vantage (IN PROGRESS)
      ├─ Kubecost (PLANNED)
      ├─ Flexera (FUTURE)
      └─ Cloudability (FUTURE)

📋 Reference
  ├─ CLI Commands
  ├─ Configuration
  ├─ API Reference
  └─ Error Codes

🚢 Deployment
  ├─ Installation
  ├─ Configuration
  ├─ Docker
  ├─ Kubernetes (FUTURE)
  ├─ CI/CD Integration
  ├─ Security
  └─ Troubleshooting

💬 Support
  ├─ FAQ
  ├─ Troubleshooting
  ├─ Contributing
  ├─ Code of Conduct
  └─ Support Channels
```

---

## Latest Updates

- **2025-10-29** - Documentation architecture planning complete
- Coming soon: Core guides and plugin documentation

---

## Contributing to Documentation

Found an error? Want to improve a guide? The documentation is open source!

See [Contributing Guide](support/contributing.html) for how to:

- Report documentation issues
- Submit improvements
- Add new examples
- Translate documentation

---

## For LLM/AI Tools

If you're an AI assistant helping someone with FinFocus, see [llms.txt](llms.txt) for a machine-readable index of all documentation.

---

**Complete Table of Contents:** [View Full TOC](TABLE-OF-CONTENTS.html)

**Last Updated:** 2025-10-29
**Version:** 1.0 (Complete Documentation)
**Status:** ✅ Core documentation complete | 🟡 Architecture diagrams in progress
