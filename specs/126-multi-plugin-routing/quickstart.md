# Quickstart: Multi-Plugin Routing

**Branch**: `126-multi-plugin-routing` | **Date**: 2026-01-24

## Overview

Multi-plugin routing enables intelligent plugin selection based on:

1. **Automatic Routing**: Uses plugin's `SupportedProviders` metadata (zero config)
2. **Declarative Routing**: YAML configuration for patterns, priorities, fallback

## Prerequisites

- FinFocus CLI installed (`finfocus --version`)
- At least two plugins installed (e.g., `aws-public`, `aws-ce`)
- Pulumi project with infrastructure code

## Quick Start (Automatic Routing)

No configuration needed! Install multiple plugins and FinFocus automatically routes:

```bash
# Install plugins
finfocus plugin install aws-public
finfocus plugin install gcp-public

# Verify plugins report their providers
finfocus plugin list
# Output:
# NAME        VERSION  PROVIDERS    CAPABILITIES
# aws-public  1.0.0    [aws]        ProjectedCosts, ActualCosts
# gcp-public  1.0.0    [gcp]        ProjectedCosts, ActualCosts

# Run cost calculation - resources automatically route to correct plugins
pulumi preview --json | finfocus cost projected --pulumi-json -
# AWS resources → aws-public
# GCP resources → gcp-public
```

## Declarative Routing Configuration

For advanced control, add routing configuration to `~/.finfocus/config.yaml`:

### Example 1: Feature-Specific Routing

Route different features to specialized plugins:

```yaml
# ~/.finfocus/config.yaml
routing:
  plugins:
    # AWS Cost Explorer for recommendations (more accurate)
    - name: aws-ce
      features:
        - Recommendations
      priority: 20

    # AWS Public for projected costs (faster, no AWS credentials needed)
    - name: aws-public
      features:
        - ProjectedCosts
        - ActualCosts
      priority: 10
```

### Example 2: Resource Pattern Routing

Route specific resource types to specialized plugins:

```yaml
routing:
  plugins:
    # EKS-specific plugin for Kubernetes costs
    - name: eks-costs
      patterns:
        - type: glob
          pattern: "aws:eks:*"
      priority: 30

    # General AWS plugin as fallback
    - name: aws-public
      patterns:
        - type: regex
          pattern: "aws:.*"
      priority: 10
```

### Example 3: Priority and Fallback

Configure priority ordering with automatic fallback:

```yaml
routing:
  plugins:
    # Primary: Try Cost Explorer first (most accurate)
    - name: aws-ce
      priority: 20
      fallback: true  # If fails, try next

    # Secondary: Public pricing as backup
    - name: aws-public
      priority: 10
      fallback: true

    # Tertiary: Local specs as last resort
    # (automatic - no config needed)
```

## Validate Configuration

Before deploying, validate your routing configuration:

```bash
finfocus config validate

# Output (success):
# ✓ Configuration valid
#
# Discovered plugins:
#   aws-ce: Recommendations (priority: 20)
#   aws-public: ProjectedCosts, ActualCosts (priority: 10)
#
# Routing rules:
#   aws:eks:* → eks-costs (pattern)
#   aws:* → aws-public (provider)
#   gcp:* → gcp-public (provider)

# Output (error):
# ✗ Configuration invalid
#
# Errors:
#   - aws-ce: plugin not found
#   - patterns[0].pattern: invalid regex: missing closing bracket
```

## View Plugin Capabilities

See what each plugin supports:

```bash
finfocus plugin list --verbose

# Output:
# NAME        VERSION  PROVIDERS    CAPABILITIES                     STATUS
# aws-public  1.0.0    [aws]        ProjectedCosts, ActualCosts      healthy
# aws-ce      1.0.0    [aws]        Recommendations, ActualCosts     healthy
# gcp-public  1.0.0    [gcp]        ProjectedCosts, ActualCosts      healthy
# eks-costs   0.5.0    [aws]        ProjectedCosts                   healthy
```

## Debug Routing Decisions

Enable debug logging to see routing decisions:

```bash
# Via flag
finfocus cost projected --debug --pulumi-json plan.json

# Via environment
export FINFOCUS_LOG_LEVEL=debug
finfocus cost projected --pulumi-json plan.json

# Example debug output:
# DBG plugin routing decision resource_type=aws:ec2/instance:Instance provider=aws matched_plugins=[aws-public] selected_plugin=aws-public priority=10
# DBG plugin routing decision resource_type=aws:eks:Cluster provider=aws matched_plugins=[eks-costs, aws-public] selected_plugin=eks-costs priority=30
```

## Common Patterns

### Multi-Cloud Setup

```yaml
routing:
  plugins:
    - name: aws-public
      priority: 10
    - name: gcp-public
      priority: 10
    - name: azure-public
      priority: 10
```

### Cost Explorer + Public Hybrid

```yaml
routing:
  plugins:
    # Cost Explorer for actual costs (uses AWS Cost Explorer API)
    - name: aws-ce
      features: [ActualCosts, Recommendations]
      priority: 20

    # Public pricing for projected costs (no AWS credentials)
    - name: aws-public
      features: [ProjectedCosts]
      priority: 10
```

### Specialized Plugin Override

```yaml
routing:
  plugins:
    # RDS-specific plugin for database costs
    - name: rds-optimizer
      patterns:
        - type: regex
          pattern: "aws:rds:.*"
      priority: 30

    # General AWS plugin for everything else
    - name: aws-public
      priority: 10
```

## Troubleshooting

### Plugin Not Receiving Requests

1. Check plugin is installed: `finfocus plugin list`
2. Verify plugin reports correct providers: `finfocus plugin list --verbose`
3. Check routing config: `finfocus config validate`
4. Enable debug logging: `--debug`

### Fallback Not Working

1. Verify `fallback: true` in config (default)
2. Check plugin is actually failing (not returning $0)
3. Review logs for fallback trigger reason

### Pattern Not Matching

1. Test pattern with `finfocus config validate`
2. For regex, ensure RE2 syntax (no backreferences)
3. For glob, use `*` not `**` (single level matching)

## Next Steps

- [Full Configuration Reference](../docs/reference/routing-config.md)
- [Plugin Development Guide](../docs/plugins/development.md)
- [Architecture Overview](../docs/architecture/routing.md)
