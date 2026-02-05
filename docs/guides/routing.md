---
title: 'Plugin Routing Configuration Guide'
description: 'Comprehensive guide to multi-plugin routing configuration for FinFocus cost calculation'
layout: default
---

**Feature**: Multi-Plugin Routing | **Status**: Stable | **Since**: v0.3.0

## Overview

FinFocus intelligently routes cost calculation requests to the most appropriate plugins based on:

1. **Provider Matching**: Automatically route resources to plugins that support their cloud provider
2. **Feature Capabilities**: Route different operations (projected costs, recommendations) to specialized plugins
3. **Resource Patterns**: Override default routing with custom glob or regex patterns
4. **Priority Ordering**: Control which plugin is preferred when multiple match
5. **Automatic Fallback**: Retry with alternative plugins when the primary fails

## Table of Contents

- [Overview](#overview)
- [How Routing Works](#how-routing-works)
  - [Layer 1: Automatic Provider-Based Routing](#layer-1-automatic-provider-based-routing-zero-configuration)
  - [Layer 2: Declarative Configuration](#layer-2-declarative-configuration-advanced-control)
- [Configuration Reference](#configuration-reference)
  - [RoutingConfig](#routingconfig)
  - [PluginRouting Fields](#pluginrouting-fields)
- [Common Configuration Patterns](#common-configuration-patterns)
- [Validation](#validation)
- [Debugging Routing Decisions](#debugging-routing-decisions)
- [Troubleshooting](#troubleshooting)
- [Performance Considerations](#performance-considerations)
- [Migration from All-Plugin Querying](#migration-from-all-plugin-querying)
- [Advanced Topics](#advanced-topics)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
- [Changelog](#changelog)

## How Routing Works

### Layer 1: Automatic Provider-Based Routing (Zero Configuration)

FinFocus automatically routes resources based on the `SupportedProviders` metadata each plugin reports:

```bash
# Install plugins - routing happens automatically
finfocus plugin install aws-public
finfocus plugin install gcp-public

# Verify what each plugin supports
finfocus plugin list
# NAME        VERSION  PROVIDERS
# aws-public  1.0.0    aws
# gcp-public  1.0.0    gcp
#
# Note: Output is tab-delimited. Column alignment may vary by terminal.

# Cost calculation automatically routes resources
finfocus cost projected --pulumi-json multi-cloud-plan.json
# aws:ec2/instance:Instance      → aws-public (provider: aws)
# gcp:compute/instance:Instance  → gcp-public (provider: gcp)
```

**Provider Extraction**: FinFocus extracts the provider from the resource type's first segment:

| Resource Type                                 | Provider     |
| --------------------------------------------- | ------------ |
| `aws:ec2/instance:Instance`                   | `aws`        |
| `gcp:compute/instance:Instance`               | `gcp`        |
| `azure:compute/virtualMachine:VirtualMachine` | `azure`      |
| `kubernetes:core/v1:Pod`                      | `kubernetes` |

**Global Plugins**: Plugins reporting `["*"]` or empty providers match ALL resources (e.g., debugging plugins).

### Layer 2: Declarative Configuration (Advanced Control)

For advanced scenarios, add routing rules to `~/.finfocus/config.yaml`:

```yaml
routing:
  plugins:
    - name: aws-ce
      features:
        - Recommendations
      priority: 20
      fallback: true

    - name: aws-public
      features:
        - ProjectedCosts
        - ActualCosts
      patterns:
        - type: glob
          pattern: 'aws:*'
      priority: 10
      fallback: true
```

## Configuration Reference

### RoutingConfig

```yaml
routing:
  plugins:
    - name: <plugin-name> # Required: Installed plugin name
      features: [...] # Optional: Limit to specific capabilities
      patterns: [...] # Optional: Resource type patterns
      priority: <number> # Optional: Selection priority (default: 0)
      fallback: <bool> # Optional: Enable fallback (default: true)
```

### PluginRouting Fields

#### `name` (required)

The plugin identifier. Must match an installed plugin name from `finfocus plugin list`.

```yaml
- name: aws-public
```

#### `features` (optional)

Limits which capabilities this plugin handles. Valid values:

- `ProjectedCosts` - Monthly cost estimates
- `ActualCosts` - Historical spending data
- `Recommendations` - Cost optimization suggestions
- `Carbon` - Carbon emission calculations
- `DryRun` - What-if analysis
- `Budgets` - Budget tracking

**Empty/omitted**: Plugin handles all features it reports.

```yaml
- name: aws-ce
  features:
    - Recommendations
    - ActualCosts
```

#### `patterns` (optional)

Resource type patterns that override automatic provider matching.

**Empty/omitted**: Use automatic provider-based routing.

```yaml
patterns:
  # Glob pattern (simpler syntax)
  - type: glob
    pattern: 'aws:eks:*'

  # Regex pattern (full RE2 syntax)
  - type: regex
    pattern: 'aws:(ec2|rds)/.*'
```

**Pattern Types**:

| Type    | Syntax            | Example      | Matches                                     |
| ------- | ----------------- | ------------ | ------------------------------------------- |
| `glob`  | `*`, `?`, `[...]` | `aws:ec2:*`  | `aws:ec2:Instance`, `aws:ec2:SecurityGroup` |
| `regex` | RE2 regex         | `aws:eks:.*` | `aws:eks:Cluster`, `aws:eks:NodeGroup`      |

**Pattern Precedence**: Patterns override automatic provider matching. If a
pattern matches, the resource routes to that plugin regardless of provider.

#### `priority` (optional)

Controls plugin selection order when multiple plugins match the same resource.

**Default**: `0` (all plugins queried in parallel)

**Behavior**:

- **Higher priority = preferred**: `priority: 30` beats `priority: 10`
- **Equal priority**: Query all matching plugins in parallel
- **Priority 0**: Special case - always query all matching

```yaml
- name: eks-costs
  priority: 30 # Highest - try first

- name: aws-ce
  priority: 20 # Medium

- name: aws-public
  priority: 10 # Lowest - fallback
```

#### `fallback` (optional)

Enables automatic retry with the next priority plugin if this one fails.

**Default**: `true`

**Fallback Triggers**:

- Connection timeout
- Plugin crash (EOF, connection reset)
- Empty result (no cost data)
- gRPC error (unavailable, internal)

**NOT Fallback Triggers**:

- Validation error (InvalidArgument) - plugin explicitly rejected
- $0 cost result - this is a valid result, not a failure

```yaml
- name: aws-ce
  fallback: true # Retry with next plugin on failure

- name: aws-public
  fallback: false # Last resort - don't fallback
```

## Common Configuration Patterns

### Pattern 1: Multi-Cloud Setup

Automatic routing works perfectly for multi-cloud infrastructure:

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

**Result**: Each resource automatically routes to its provider's plugin.

### Pattern 2: Feature-Specific Routing

Route different features to specialized plugins:

```yaml
routing:
  plugins:
    # AWS Cost Explorer for recommendations (uses AWS API)
    - name: aws-ce
      features:
        - Recommendations
        - ActualCosts
      priority: 20

    # AWS Public for projected costs (no credentials needed)
    - name: aws-public
      features:
        - ProjectedCosts
      priority: 10
```

**Result**:

- `finfocus cost projected` → aws-public
- `finfocus cost recommendations` → aws-ce
- `finfocus cost actual` → aws-ce

### Pattern 3: Specialized Plugin Override

Route specific resource types to specialized plugins:

```yaml
routing:
  plugins:
    # EKS-specific plugin for Kubernetes costs
    - name: eks-costs
      patterns:
        - type: glob
          pattern: 'aws:eks:*'
      priority: 30

    # RDS-specific plugin for database costs
    - name: rds-optimizer
      patterns:
        - type: regex
          pattern: 'aws:rds:.*'
      priority: 30

    # General AWS plugin for everything else
    - name: aws-public
      priority: 10
```

**Result**:

- `aws:eks:Cluster` → eks-costs (pattern match)
- `aws:rds:Instance` → rds-optimizer (pattern match)
- `aws:ec2:Instance` → aws-public (provider match)

### Pattern 4: Priority Chain with Fallback

Configure primary plugin with automatic fallback to backup:

```yaml
routing:
  plugins:
    # Primary: Try AWS Cost Explorer first (most accurate)
    - name: aws-ce
      priority: 20
      fallback: true

    # Secondary: Try AWS Public as backup
    - name: aws-public
      priority: 10
      fallback: true

    # Tertiary: Local specs as last resort (automatic)
```

**Execution Flow**:

1. Try `aws-ce` (priority 20)
2. If fails and `fallback: true` → try `aws-public` (priority 10)
3. If fails and `fallback: true` → try local specs (built-in)
4. If no specs → return "no cost data available"

## Validation

### Eager Validation (Before Deployment)

Validate your routing configuration before using it:

```bash
finfocus config validate

# Success output:
✓ Configuration valid

Discovered plugins:
  aws-ce: Recommendations, ActualCosts (priority: 20)
  aws-public: ProjectedCosts (priority: 10)

Routing rules:
  aws:eks:* → eks-costs (pattern, priority: 30)
  aws:* → aws-public (provider, priority: 10)

# Error output:
✗ Configuration invalid

Errors:
  - aws-ce: plugin not found (install with: finfocus plugin install aws-ce)
  - patterns[0].pattern: invalid regex: missing closing bracket

Warnings:
  - aws-public: feature 'Carbon' not supported by plugin
```

### Lazy Validation (At Runtime)

Validation happens automatically on first use. Invalid patterns are skipped with warnings logged.

## Debugging Routing Decisions

### Enable Debug Logging

See exactly which plugin was selected and why:

```bash
# Via flag
finfocus cost projected --debug --pulumi-json plan.json

# Via environment
export FINFOCUS_LOG_LEVEL=debug
finfocus cost projected --pulumi-json plan.json

# Example debug output:
DBG plugin routing decision resource_type=aws:ec2/instance:Instance provider=aws matched_plugins=[aws-public] selected_plugin=aws-public priority=10 reason=provider
DBG plugin routing decision resource_type=aws:eks:Cluster provider=aws matched_plugins=[eks-costs, aws-public] selected_plugin=eks-costs priority=30 reason=pattern
```

### View Plugin Capabilities

Check what each plugin supports:

```bash
finfocus plugin list --verbose

# Output:
NAME        VERSION  PROVIDERS    CAPABILITIES                     STATUS
aws-public  1.0.0    aws          ProjectedCosts, ActualCosts      healthy
aws-ce      1.0.0    aws          Recommendations, ActualCosts     healthy
gcp-public  1.0.0    gcp          ProjectedCosts, ActualCosts      healthy
```

## Troubleshooting

### Plugin Not Receiving Requests

**Symptoms**: Plugin installed but never queried.

**Solutions**:

1. Check plugin is installed:

   ```bash
   finfocus plugin list
   ```

2. Verify plugin reports correct providers:

   ```bash
   finfocus plugin list --verbose
   ```

3. Check routing configuration:

   ```bash
   finfocus config validate
   ```

4. Enable debug logging:

   ```bash
   finfocus cost projected --debug --pulumi-json plan.json
   ```

### Fallback Not Working

**Symptoms**: Primary plugin fails but fallback doesn't trigger.

**Solutions**:

1. Verify `fallback: true` in config (default):

   ```yaml
   - name: primary-plugin
     fallback: true # Must be true
   ```

2. Check plugin is actually failing (not returning $0):

   ```bash
   finfocus cost projected --debug --pulumi-json plan.json
   ```

   Look for `ERR` or `WARN` logs indicating failure.

3. Review fallback trigger reason:

   ```bash
   finfocus cost projected --debug --pulumi-json plan.json
   # Look for: "plugin failed, trying fallback"
   ```

### Pattern Not Matching

**Symptoms**: Resource pattern doesn't route to expected plugin.

**Solutions**:

1. Validate pattern syntax:

   ```bash
   finfocus config validate
   ```

2. For regex, ensure RE2 syntax (no backreferences):

   ```yaml
   # CORRECT (RE2 syntax)
   - type: regex
     pattern: 'aws:(ec2|rds)/.*'

   # INCORRECT (backreferences not supported)
   - type: regex
     pattern: "aws:(\\w+)/\\1" # \1 backreference fails
   ```

3. For glob, use `*` not `**` (single-level matching):

   ```yaml
   # CORRECT
   - type: glob
     pattern: 'aws:eks:*'

   # INCORRECT (** not supported by filepath.Match)
   - type: glob
     pattern: 'aws:eks:**'
   ```

4. Enable debug logging to see pattern matching:

   ```bash
   finfocus cost projected --debug --pulumi-json plan.json
   # Look for: "pattern match" events at TRACE level
   ```

### All Plugins Failing

**Symptoms**: All plugins fail, falling back to specs or "no cost data".

**Solutions**:

1. Check plugin health:

   ```bash
   finfocus plugin list --verbose
   ```

2. Verify plugin logs for errors:

   ```bash
   # Check logs in ~/.finfocus/logs/ if logging enabled
   ```

3. Test individual plugin:

   ```bash
   finfocus plugin inspect <plugin-name> <resource-type>
   ```

4. Check network connectivity (for cloud API plugins).

## Performance Considerations

### Routing Overhead

Routing decisions are designed to be fast:

- **Provider extraction**: ~10ns per resource (simple string split)
- **Glob pattern match**: ~100ns per resource (filepath.Match)
- **Regex pattern match**: ~200ns per resource (compiled regex cache)
- **Priority sorting**: ~O(n log n) where n = matching plugins

**Target**: <10ms routing overhead per request (SC-002)

**Typical**: 100-200 microseconds for 10 patterns × 100 resources

### Pattern Cache

Regex patterns are compiled once at config load and cached:

```go
// Compiled at config load
regex, err := regexp.Compile(pattern)

// Cached for subsequent matches
cached[pattern] = regex
```

**Benefit**: Avoids recompilation on each resource evaluation.

## Migration from All-Plugin Querying

### Legacy Behavior (Pre-v0.3.0)

Before multi-plugin routing, FinFocus queried ALL plugins for ALL resources:

```bash
# Old behavior: queries aws-public, gcp-public, recorder for EVERY resource
finfocus cost projected --pulumi-json plan.json
```

### New Behavior (v0.3.0+)

With routing, only matching plugins are queried:

```bash
# New behavior: queries only matching plugins
finfocus cost projected --pulumi-json plan.json
# aws:ec2:Instance → queries only aws-public
# gcp:compute:Instance → queries only gcp-public
```

### Backward Compatibility

**No configuration**: Automatic provider-based routing (backward compatible).

**Existing configs**: Routing is additive - configs without `routing:` key work unchanged.

```yaml
# Legacy config (still works)
plugins:
  aws-public:
    region: us-east-1
# No routing config = automatic routing based on providers
```

## Advanced Topics

### Global Plugins (Wildcard Providers)

Plugins reporting `["*"]` or empty providers match ALL resources:

```yaml
# Example: recorder plugin for debugging
routing:
  plugins:
    - name: recorder
      priority: 0 # Query alongside other plugins
```

**Use Cases**:

- Debugging/logging plugins
- Audit/compliance plugins
- Testing/validation plugins

### Equal Priority Behavior

When multiple plugins have the same priority, ALL are queried in parallel:

```yaml
routing:
  plugins:
    - name: aws-public
      priority: 10
    - name: aws-ce
      priority: 10 # Same priority
```

**Result**: Both `aws-public` and `aws-ce` queried concurrently, results merged.

### Source Attribution

Results include which plugin provided the data:

```json
{
  "resource": "aws:ec2/instance:Instance",
  "monthlyCost": 375.0,
  "source": "aws-public", // Attribution field
  "currency": "USD"
}
```

**Use Cases**:

- Debugging which plugin responded
- Comparing results from multiple plugins
- Audit trails for cost data sources

## Best Practices

1. **Start Simple**: Begin with automatic routing (zero config), add declarative rules only when needed.

2. **Validate Early**: Run `finfocus config validate` before deploying routing changes.

3. **Use Priorities**: Assign clear priority ordering to avoid ambiguity.

4. **Enable Fallback**: Keep `fallback: true` (default) for resilience.

5. **Debug Liberally**: Use `--debug` flag to understand routing decisions.

6. **Pattern Specificity**: Make patterns as specific as needed - general patterns can cause unexpected routing.

7. **Monitor Performance**: Ensure routing overhead stays <10ms per request.

## Related Documentation

- [Plugin Development Guide](../plugins/plugin-development.md) - How to build plugins with routing support
- [CLI Commands Reference](../reference/cli-commands.md) - All CLI commands with routing options
- [Configuration Schema](../reference/config-schema.md) - Complete config.yaml schema
- [Architecture Overview](../architecture/system-overview.md) - How routing fits in the system

## Changelog

- **v0.3.0**: Initial multi-plugin routing release
  - Automatic provider-based routing
  - Declarative pattern/feature/priority configuration
  - Validation command and debug logging
