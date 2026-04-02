---
title: Pulumi Analyzer Integration
description: Technical details of the FinFocus analyzer gRPC integration
layout: default
---

FinFocus integrates with Pulumi's analyzer framework to provide real-time cost
estimates during `pulumi preview` operations.

## Architecture

The analyzer is a gRPC server that the Pulumi engine invokes during the resource
lifecycle. It provides "advisory" diagnostics that do not block deployment but
inform the user of potential costs.

For setup instructions, see the [Analyzer Setup Guide](getting-started/analyzer-setup.md).

## How It Works

The analyzer is invoked automatically by Pulumi during preview. It:

1. Receives resource information via gRPC
2. Calculates costs using the pricing engine and plugins
3. Returns cost diagnostics that appear in the Pulumi output

## Key Technical Details

### Binary Naming Convention

Pulumi looks for `pulumi-analyzer-policy-<runtime>` on PATH. The binary must be in
your PATH — having it only in the policy pack directory is not sufficient. Since we
use `runtime: finfocus` in `PulumiPolicy.yaml`, the binary must be named:

```text
pulumi-analyzer-policy-finfocus
```

### Handshake Protocol

When Pulumi starts the analyzer:

1. Pulumi executes the binary.
2. The binary detects it is being run as an analyzer (by checking `os.Args[0]`).
3. The binary starts a gRPC server on a random available port.
4. Binary prints the port number to `stdout` (ONLY the port, nothing else).
5. Pulumi connects to that port via gRPC.
6. **Important**: All logging must go to `stderr` to avoid breaking the handshake.

### RPC Methods

The analyzer implements these Pulumi Analyzer gRPC methods:

| Method            | Purpose                                     |
| ----------------- | ------------------------------------------- |
| `Handshake`       | Acknowledge connection from Pulumi engine   |
| `GetAnalyzerInfo` | Return analyzer metadata and policy info    |
| `GetPluginInfo`   | Return plugin version                       |
| `ConfigureStack`  | Receive stack context before analysis       |
| `Analyze`         | Analyze single resource, return diagnostics |
| `AnalyzeStack`    | Called at end, return summary diagnostic    |
| `Cancel`          | Handle graceful shutdown                    |

### Diagnostic Workflow

1. `ConfigureStack` is called once at start (clears cost cache).
2. `Analyze` is called for each resource (returns per-resource costs, caches them).
3. `AnalyzeStack` is called once at end (returns summary using cached costs).

This prevents duplicate diagnostics in the output and ensures the summary is
accurate based on the resources analyzed in the current run.

### Enforcement Level

All diagnostics use `ADVISORY` enforcement, meaning they never block deployments.
Costs are informational only.

## Internal Types

Pulumi internal resources (Stack, providers) are handled specially:

- Type prefix: `pulumi:`
- Cost: $0.00
- Message: "Internal Pulumi resource (no cloud cost)"

## Isolating Plugins in Analyzer Mode

When finfocus runs as a Pulumi policy pack, routing configuration is not applied.
The only supported mechanism for controlling which plugins are loaded is `FINFOCUS_HOME`.

When `FINFOCUS_HOME` is set, finfocus uses `$FINFOCUS_HOME/plugins/` as its entire
plugin root. Any plugin absent from that directory is never loaded.

> **Warning: Directory-level symlinks are not supported**
> (issue [#750](https://github.com/rshade/finfocus/issues/750))
>
> The registry silently skips directory-level symlinks. If you symlink the entire
> plugin version directory, the plugin will not be found. Use file-level symlinks
> (symlink the binary itself) until issue #750 is resolved.

### Step-by-step: create an isolated plugin home

**Step 1**: Create real plugin directories (not symlinks):

```bash
mkdir -p ~/.finfocus/demo/plugins/aws-public/v0.1.5
```

**Step 2**: Symlink only the plugin binary (file-level):

```bash
ln -sf ~/.finfocus/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public-us-east-1 \
    ~/.finfocus/demo/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public-us-east-1
```

Repeat Step 1 and Step 2 for each plugin you want to include.

**Step 3**: Run `pulumi preview` with the isolated home:

```bash
FINFOCUS_HOME=~/.finfocus/demo \
  pulumi preview --policy-pack /path/to/finfocus-policy-pack
```

Only the plugins present in `~/.finfocus/demo/plugins/` will be loaded. If the
directory contains no plugins, finfocus returns zero cost data (it does **not**
fall back to `~/.finfocus`).

### Environment variable precedence

`FINFOCUS_HOME` takes highest precedence over `PULUMI_HOME/finfocus` and `~/.finfocus`.
See [Configuration Reference](reference/config-schema.md) for the full precedence order.

## See Also

- [Analyzer Setup Guide](getting-started/analyzer-setup.md)
- [CLI Commands Reference](reference/cli-commands.md)
