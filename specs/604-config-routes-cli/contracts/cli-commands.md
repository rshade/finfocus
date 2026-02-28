# CLI Command Contracts: Config Routes

**Feature**: 604-config-routes-cli

## Command Tree

```text
finfocus config routes          # Parent command (no action)
finfocus config routes list     # Display routing rules
finfocus config routes test     # Simulate plugin selection
```

## Contract: `config routes list`

### List Invocation

```text
finfocus config routes list [--output table|json]
```

### List Arguments

None.

### List Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | `table` | Output format: `table` or `json` |

### List Behavior

1. Load effective configuration (project-local merged over global)
2. Check if `Routing` is nil (automatic mode) or configured
3. If automatic: display informational message and exit 0
4. If configured: display routing rules sorted by priority (descending)

### List Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (rules displayed or automatic mode indicated) |
| 1 | Error loading configuration |

### List Table Output Schema

```text
PLUGIN ROUTING RULES

  PRIORITY  PLUGIN      FEATURES        PATTERNS         FALLBACK
  --------  ------      --------        --------         --------
  {int}     {string}    {csv|"(all)"}   {type:pat|"(all)"} {yes|no}

  Source: {config_path} ({project|global})
```

### List JSON Output Schema

```json
{
  "mode": "configured|automatic",
  "config_path": "/path/to/config.yaml",
  "source": "project|global",
  "rules": [
    {
      "plugin": "string",
      "priority": 0,
      "features": ["string"],
      "patterns": ["type:pattern"],
      "fallback": true
    }
  ]
}
```

## Contract: `config routes test`

### Test Invocation

```text
finfocus config routes test <resource-type> [region] [--output table|json]
```

### Test Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `resource-type` | Yes | Pulumi resource type (e.g., `aws:ec2:Instance`) |
| `region` | No | Cloud region (e.g., `us-east-1`) |

### Test Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | `table` | Output format: `table` or `json` |

### Test Behavior

1. Load effective configuration
2. Parse resource type and optional region from arguments
3. Extract provider from resource type
4. If no routing config (automatic mode): display informational message
   explaining that all plugins matching the provider would be queried
5. If routing configured:
   a. Create synthetic `pluginhost.Client` instances from config plugin names
   b. Create `DefaultRouter` with config and synthetic clients
   c. For each feature type, call `SelectPlugins()` with a synthetic
      `ResourceDescriptor`
   d. Display the match chain (all matching plugins ranked by priority)
   e. Display per-feature assignments (highest priority match per feature)

### Test Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (matches displayed, no-match indicated, or automatic mode) |
| 1 | Error loading configuration or missing resource-type argument |

### Test Table Output Schema

```text
Plugin selection for {resource-type} (region: {region}):

  #  PLUGIN      PRIORITY  MATCH REASON  SOURCE
  -  ------      --------  ------------  ------
  {n} {string}   {int}     {reason}      {source}

Feature availability:
  {FeatureName}:  {plugin} (priority {n})
```

Match reasons: `pattern`, `automatic`, `global`, `no_match`
Sources: `config`, `automatic`

### Test JSON Output Schema

```json
{
  "resource_type": "aws:ec2:Instance",
  "region": "us-east-1",
  "provider": "aws",
  "mode": "configured|automatic",
  "matches": [
    {
      "rank": 1,
      "plugin": "string",
      "priority": 10,
      "match_reason": "pattern",
      "source": "config",
      "fallback": false
    }
  ],
  "features": {
    "ProjectedCosts": "aws-public",
    "ActualCosts": "aws-ce",
    "Recommendations": "aws-ce",
    "Carbon": "recorder",
    "DryRun": "recorder",
    "Budgets": "recorder"
  }
}
```

## Contract: `config routes` (parent)

### Parent Invocation

```text
finfocus config routes
```

### Parent Behavior

Displays help text listing available subcommands (`list`, `test`).
No `RunE` function — delegates to subcommands only.

### Parent Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Help text displayed |
