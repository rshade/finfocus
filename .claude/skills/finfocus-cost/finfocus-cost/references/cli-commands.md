# CLI Command Reference

## Table of Contents

- [Command Hierarchy](#command-hierarchy)
- [Global Flags](#global-flags)
- [Cost Projected](#cost-projected)
- [Cost Actual](#cost-actual)
- [Cost Recommendations](#cost-recommendations)
- [Budgets](#budgets)
- [Plugin Management](#plugin-management)
- [Analyzer](#analyzer)
- [Auto-Detection](#auto-detection)

## Command Hierarchy

```text
finfocus
├── cost
│   ├── projected         # Estimate from Pulumi preview
│   ├── actual            # Historical costs with time ranges
│   ├── estimate          # Cost estimation
│   └── recommendations   # Optimization suggestions
│       ├── dismiss       # Dismiss a recommendation
│       ├── snooze        # Dismiss until a date
│       ├── undismiss     # Undo dismissal
│       └── history       # View dismissal history
├── overview              # Dashboard view (alias: ov)
├── plugin
│   ├── init              # Scaffold new plugin
│   ├── install           # Install plugin
│   ├── update            # Update plugin
│   ├── remove            # Remove plugin
│   ├── list              # List installed
│   ├── validate          # Validate installations
│   ├── inspect           # Plugin details
│   ├── certify           # Certification tests
│   └── conformance       # Conformance tests
├── config
│   ├── init / get / set / list / validate
└── analyzer
    └── serve             # Pulumi Analyzer gRPC server
```

## Global Flags

| Flag | Purpose |
|------|---------|
| `--debug` | Enable debug logging |
| `--project-dir` | Pulumi project directory for config resolution |

## Cost Projected

```bash
finfocus cost projected [flags]
```

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--pulumi-json` | string | No* | Path to `pulumi preview --json` output |
| `--pulumi-state` | string | No* | Path to Pulumi state file |
| `--adapter` | string | No | Override plugin adapter |
| `--output` | string | No | Output format: table, json, ndjson |
| `--filter` | string | No | Resource filter expression |

*Auto-detects if neither provided.

## Cost Actual

```bash
finfocus cost actual [flags]
```

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--pulumi-json` | string | No* | Pulumi JSON path |
| `--from` | string | Yes | Start date (2006-01-02 or RFC3339) |
| `--to` | string | No | End date (defaults to now) |
| `--adapter` | string | No | Plugin adapter |
| `--output` | string | No | Output format |
| `--group-by` | string | No | Grouping: resource, type, provider, daily, monthly |
| `--filter` | string | No | Resource filter |

## Cost Recommendations

```bash
finfocus cost recommendations [flags]
```

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--pulumi-json` | string | No* | Pulumi JSON path |
| `--filter` | string | No | Filter expression, e.g. `action=RIGHTSIZE,TERMINATE` |
| `--output` | string | No | Output format |

## Budgets

There is no `cost budget` command. `cost projected` and `cost actual` print
budget status and take these budget flags:

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--budget-scope` | string | No | Scopes to show: global, provider, provider=aws, tag, type |
| `--exit-on-threshold` | bool | No | Exit non-zero when a threshold is exceeded |
| `--exit-code` | int | No | Exit code for an exceeded threshold (default 1) |

## Plugin Management

```bash
finfocus plugin list              # List installed plugins
finfocus plugin validate          # Validate all plugins
finfocus plugin install <name>    # Install a plugin
finfocus plugin remove <name>     # Remove a plugin
finfocus plugin inspect <name>    # Show plugin details
```

Plugin directory: `~/.finfocus/plugins/<name>/<version>/`

## Analyzer

```bash
finfocus analyzer serve
```

- Starts gRPC server on random TCP port
- Prints ONLY port number to stdout (Pulumi handshake)
- All logging to stderr
- Graceful shutdown on SIGINT/SIGTERM
- ADVISORY enforcement only (never blocks deployments)

## Auto-Detection

When no `--pulumi-json` or `--pulumi-state` provided:

1. Searches for `Pulumi.yaml` in current directory and parents
2. Runs `pulumi preview --json` to get current plan
3. Falls back to error with guidance: "Use --pulumi-json or --pulumi-state"

Requires `pulumi` CLI in PATH.
