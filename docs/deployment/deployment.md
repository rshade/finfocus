---
title: Deployment Overview
layout: default
---

FinFocus can be deployed on developer workstations, in CI/CD pipelines, or within
containerized environments. This guide helps developers, DevOps engineers, and platform teams
choose the right approach and connect to detailed setup instructions.

## How to Use This Guide

- Start with the decision guide to pick a deployment model.
- Jump to the deployment method for an overview and quick start.
- Use the specialized guides for step-by-step configuration.

Related guides:

- [Configuration](configuration.md)
- [Docker](docker.md)
- [CI/CD Integration](cicd-integration.md)
- [Security](security.md)
- [Troubleshooting](troubleshooting.md)

## Section Overview

- Deployment goals and flexibility
- Decision matrix and recommendations
- Deployment methods (local, Docker, CI/CD)
- Production and enterprise guidance
- Quick reference and next steps

## Deployment Flexibility

FinFocus is flexible in how it runs and where it stores state:

- Run as a standalone CLI or as a Pulumi Analyzer policy pack
- Use local config files, environment variables, or container mounts
- Adopt plugins and pricing specs from local files or shared registries

## Deployment Decision Guide

Use this matrix to choose the right deployment model.

| Deployment method  | Best use case                        | Complexity | Isolation | Automation | Team size |
| ------------------ | ------------------------------------ | ---------- | --------- | ---------- | --------- |
| Local workstation  | Ad hoc analysis, developer workflows | Low        | Low       | Manual     | 1-5       |
| Docker container   | Repeatable runtime, shared tooling   | Medium     | Medium    | Scripted   | 2-20      |
| CI/CD pipeline     | Automated gates, policy enforcement  | Medium     | Low       | High       | 5+        |
| Enterprise rollout | Standardized controls, auditability  | High       | High      | High       | 20+       |

### Scenario Recommendations

- **Individual developer**: Start with the local CLI for rapid feedback.
- **Small team**: Use Docker for repeatable runs and shared configs.
- **CI/CD gate**: Automate checks on pull requests with CI/CD integration.
- **Enterprise**: Combine CI/CD with centralized config and plugin governance.

### Quick Guidelines

- Prefer local CLI for interactive cost exploration.
- Use Docker when you need consistent runtime or air-gapped execution.
- Use CI/CD to enforce budgets and capture audit trails.
- Plan enterprise deployments around shared plugins and policy packs.

## Deployment Methods

### Local Development

Use this model for individual developers or small teams validating costs during daily work.

#### Standalone CLI

- Install the CLI and configure `~/.finfocus/config.yaml`.
- Plugins live in `~/.finfocus/plugins` and specs in `~/.finfocus/specs`.
- Run `finfocus cost projected` against Pulumi plan JSON output.

#### Pulumi Analyzer Policy Pack

- Policy pack directory: `~/.finfocus/analyzer`.
- Runtime config: `~/.finfocus/analyzer/PulumiPolicy.yaml`.
- Binary name: `pulumi-analyzer-policy-finfocus`.

#### Local Quick Start

```bash
finfocus --version
finfocus plugin list
finfocus cost projected --pulumi-json plan.json
```

#### Verification

```bash
pulumi preview --policy-pack ~/.finfocus/analyzer
```

See [Installation](../getting-started/installation.md) and
[Analyzer Setup](../getting-started/analyzer-setup.md) for details.

### Docker Containers

Use containers for repeatable, isolated execution in shared environments.

- Official image: Alpine base, non-root user (UID 1001), multi-stage builds.
- Mount `~/.finfocus` for config, plugins, and specs.
- Mount the workspace for Pulumi JSON input.

#### Docker Quick Start

```bash
docker run --rm \
  -v ~/.finfocus:/home/finfocus/.finfocus \
  -v $(pwd):/workspace \
  ghcr.io/rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json
```

Use Docker when you need a consistent runtime or to run in air-gapped systems.
See the [Docker Guide](docker.md) for full configuration details.

### CI/CD Pipelines

Use CI/CD to automate cost analysis in pull requests and deployments.

#### Pattern

1. Generate Pulumi plan JSON (`pulumi preview --json > plan.json`).
2. Run `finfocus cost projected` for cost estimation.
3. Enforce policy thresholds and budget checks.

#### Environment Variables

- `FINFOCUS_LOG_LEVEL`: Sets logging verbosity.
- `FINFOCUS_CONFIG_FILE`: Path to a custom configuration file.
- `FINFOCUS_PLUGIN_DIR`: Directory used to cache plugins.

#### Credentials

- Use OIDC or short-lived tokens.
- Store secrets in GitHub Actions or GitLab CI variables.

See [CI/CD Integration](cicd-integration.md) for platform-specific examples.

## Deployment Architecture

A typical deployment uses the same core components across environments:

- **CLI**: The `finfocus` binary that runs cost analysis and plugin management.
- **Plugins**: Provider-specific pricing sources (for example, Vantage).
- **Specs**: Local pricing data stored under `~/.finfocus/specs`.
- **Config**: `config.yaml` or environment variables for runtime settings.

For a visual overview, see the [Deployment Architecture Diagram](deployment-architecture.md).
For configuration reference, see the [Configuration Guide](configuration.md).

## Production Considerations

### Configuration Management

- Config priority: CLI flags, environment variables, then `config.yaml`.
- Use environment variables for secrets and dynamic settings.
- Centralize config templates for consistent defaults.

### Logging and Observability

- Set `logging.level` or `FINFOCUS_LOG_LEVEL` for verbosity.
- Use `logging.format: json` for structured logs.
- Capture CLI output as CI artifacts for audit trails.

### Security Best Practices

- Follow least-privilege credentials and OIDC in CI.
- Validate plugins with `finfocus plugin validate`.
- Use the guidance in [Security](security.md).

### Container Monitoring

- Use container health checks and log aggregation.
- Monitor plugin failures and retry alerts.

See [Configuration](configuration.md) and [Troubleshooting](troubleshooting.md).

## Enterprise Deployment

Enterprise deployment guides are coming soon. Today, you can still build a
secure rollout with the current capabilities below.

### Current Capabilities

- Centralized configuration with environment templates
- Shared plugin registries and approval workflows
- Policy packs stored in version control
- CI/CD-driven audit trails for cost reports

### Planned Features

- Signed plugin distribution and checksum validation
- Hosted plugin registry with access controls
- Policy pack governance and reporting dashboards

### Evaluation Guidance

- Start with a pilot stack and a small set of trusted plugins.
- Scale to more teams once pricing specs are validated.
- Integrate cost outputs into finance or platform reporting.

See the [Architecture Overview](../architecture/system-overview.md) for context.

## Quick Reference

| Method               | Use case                | Configuration              | Documentation                                          |
| -------------------- | ----------------------- | -------------------------- | ------------------------------------------------------ |
| Local CLI            | Interactive cost checks | `~/.finfocus/config.yaml`  | [Installation](../getting-started/installation.md)     |
| Analyzer policy pack | Pulumi previews         | `~/.finfocus/analyzer`     | [Analyzer Setup](../getting-started/analyzer-setup.md) |
| Docker               | Consistent runtime      | `/home/finfocus/.finfocus` | [Docker](docker.md)                                    |
| CI/CD                | Automated checks        | Env vars, secrets          | [CI/CD Integration](cicd-integration.md)               |

### Common Scenarios

- **Developer laptop**: Use local CLI, then add analyzer for previews.
- **Shared build agent**: Run Docker with mounted plugins and specs.
- **Pull request gates**: Use CI/CD with OIDC and cached plugins.

## Next Steps

- Install and configure the CLI from [Installation](../getting-started/installation.md).
- Set up analyzer previews with [Analyzer Setup](../getting-started/analyzer-setup.md).
- Configure Docker runs via [Docker](docker.md).
- Add pipeline checks via [CI/CD Integration](cicd-integration.md).
- If you hit issues, start with [Troubleshooting](troubleshooting.md).
