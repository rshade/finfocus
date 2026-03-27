---
name: finfocus-routing
description: >
  Configure and debug FinFocus intelligent plugin routing. Use when setting up
  multi-plugin routing, configuring priority and fallback rules, writing pattern
  matching for resource types, testing route selection, or debugging why a plugin
  isn't receiving cost queries. Triggers on: "configure routing", "plugin priority",
  "fallback chain", "route test", "multi-region routing", "pattern matching",
  "config routes", "plugin selection", "routing config", or any task involving
  finfocus plugin routing configuration and debugging.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Plugin Routing

Route cost queries to the right plugin based on provider, region, patterns,
and priority.

## No Config = No Routing

Without a `routing:` section in config.yaml, the engine queries **all** plugins
for every resource. Add routing config only when you need control.

## Config Format

```yaml
# ~/.finfocus/config.yaml (or $PROJECT/.finfocus/config.yaml)
routing:
  plugins:
    - name: aws-public        # Must match installed plugin name
      priority: 10            # Higher = preferred (default: 0)
      features:               # Limit capabilities (default: all)
        - ProjectedCosts
      patterns:               # Declarative type matching (optional)
        - type: glob
          pattern: "aws:ec2:*"
      fallback: true          # Try next on failure (default: true)
```

## Priority and Fallback

- **Higher number = higher priority** (sorted descending, stable sort)
- **Equal priority** (all 0) = plugins queried in **parallel**
- **$0.00 is a VALID result** — does NOT trigger fallback
- Only nil/empty results or errors trigger fallback to next plugin
- `InvalidArgument` does NOT trigger fallback (plugin explicitly rejected)

## Features

| Feature | gRPC Method | Capability |
|---------|-------------|------------|
| `ProjectedCosts` | `GetProjectedCost` | `PLUGIN_CAPABILITY_PROJECTED_COSTS` |
| `ActualCosts` | `GetActualCost` | `PLUGIN_CAPABILITY_ACTUAL_COSTS` |
| `Recommendations` | `GetRecommendations` | `PLUGIN_CAPABILITY_RECOMMENDATIONS` |
| `Carbon` | `GetCarbonFootprint` | `PLUGIN_CAPABILITY_CARBON` |
| `DryRun` | `PerformDryRun` | `PLUGIN_CAPABILITY_DRY_RUN` |
| `Budgets` | `GetBudgetStatus` + 3 others | `PLUGIN_CAPABILITY_BUDGETS` |
| `BatchCost` | `BatchCost` | `PLUGIN_CAPABILITY_BATCH_COST` |

Empty features list = all features enabled. Feature names are case-sensitive.

## Pattern Matching

Two types for declarative resource type matching:

- **glob**: `filepath.Match` semantics, `*` matches across `/` boundaries.
  Example: `aws:ec2:*` matches `aws:ec2:Instance`
- **regex**: RE2 syntax.
  Example: `aws:(ec2|rds)/.*` matches `aws:ec2/instance:Instance`

Patterns take **highest precedence** over automatic provider matching.

## Matching Precedence

1. **Declarative patterns** — glob/regex against resource type
2. **Automatic provider** — `SupportedProviders` from `GetPluginInfo`
3. **Global plugins** — empty or `["*"]` SupportedProviders

Internal Pulumi types (`pulumi:*`) skip automatic matching unless explicitly
matched by a pattern.

## Region Matching

Region extracted from resource properties in order: `region` >
`availabilityZone` > `availability_zone` > `location` > ARN.

- AZs normalized: `us-west-2a` → `us-west-2`, `us-central1-a` → `us-central1`
- Empty region on either side = wildcard (matches anything)
- Comparison is case-insensitive

## CLI Commands

```bash
# Show effective routing rules
finfocus config routes list
finfocus config routes list --output json

# Simulate plugin selection (no plugin contact needed)
finfocus config routes test aws:ec2:Instance
finfocus config routes test aws:ec2:Instance us-east-1
finfocus config routes test aws:ec2:Instance --output json
```

## Common Scenarios

See [references/routing-scenarios.md](references/routing-scenarios.md) for
multi-region, feature-based, pattern-based, and multi-cloud configurations
with complete YAML examples.

## Validation

```bash
# Validation runs automatically on config load
# Errors are blocking, warnings are non-blocking
```

- **Errors**: empty name, missing plugin, invalid regex, negative priority,
  invalid pattern type, empty pattern
- **Warnings**: duplicate plugin config, unknown feature names
