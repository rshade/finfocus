---
name: finfocus-analyzer-setup
description: >
  Install and configure FinFocus as a Pulumi Analyzer for inline cost estimation
  during pulumi preview. Use when setting up the analyzer, configuring the policy
  pack, debugging analyzer issues, or enabling zero-click cost estimates in Pulumi
  workflows. Triggers on: "set up analyzer", "configure analyzer", "install analyzer",
  "Pulumi preview costs", "inline cost estimates", "policy pack setup",
  "analyzer integration", "zero-click cost estimation", "analyzer check",
  or any task involving finfocus Pulumi Analyzer configuration.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Analyzer Setup

Inline cost estimation during `pulumi preview` via the Pulumi Analyzer protocol.
All diagnostics are ADVISORY — never blocks deployments.

## Prerequisites

```bash
finfocus --version          # FinFocus CLI installed
pulumi version              # Pulumi CLI installed
finfocus plugin list        # At least one cost plugin installed
```

## Quick Setup

```bash
finfocus analyzer install           # Install + create policy pack
finfocus analyzer check             # Verify all 4 checks pass

# Add to PATH (required for Pulumi to discover the binary)
export PATH="$HOME/.finfocus/analyzer:$PATH"

# Run preview with costs
pulumi preview --policy-pack ~/.finfocus/analyzer
```

For persistent PATH, add the export to `~/.bashrc` or `~/.zshrc`.

**Windows (PowerShell):** Add to your PowerShell profile (`$PROFILE`):

```powershell
$env:PATH = "$env:USERPROFILE\.finfocus\analyzer;$env:PATH"
```

Restart PowerShell for changes to take effect.

## What `analyzer install` Does

1. Creates `~/.pulumi/plugins/analyzer-finfocus-v{VERSION}/pulumi-analyzer-finfocus`
   (symlink on Unix, copy on Windows)
2. Creates `~/.finfocus/analyzer/PulumiPolicy.yaml` (runtime: finfocus)
3. Creates `~/.finfocus/analyzer/pulumi-analyzer-policy-finfocus` (binary reference)

Use `--force` to reinstall after upgrading finfocus. Use `--target-dir` to
override the Pulumi plugin directory.

## Verification

```bash
finfocus analyzer check                # Table output
finfocus analyzer check --output json  # JSON for CI
```

Runs 4 sequential checks (later checks skip if earlier ones fail):

| Check | Verifies |
|-------|----------|
| `policy_pack_dir` | `~/.finfocus/analyzer/` exists |
| `pulumi_policy_yaml` | `PulumiPolicy.yaml` has `runtime: finfocus` |
| `binary_in_path` | `pulumi-analyzer-policy-finfocus` on PATH |
| `grpc_smoke_test` | `analyzer serve` starts and responds to RPC |

## Expected Output

```text
Policies:
    finfocus@v0.3.3 (local: ~/.finfocus/analyzer)
        - [advisory]  cost-estimate  (aws:ec2/instance:Instance: my-instance)
          Estimated Monthly Cost: $7.50 USD (source: finfocus-plugin-aws)
        - [advisory]  stack-cost-summary  (pulumi:pulumi:Stack: my-stack)
          Total Estimated Monthly Cost: $7.50 USD (1 resources analyzed)
```

## CI/CD Usage

```bash
export PULUMI_POLICY_PACK_PATH="$HOME/.finfocus/analyzer"
pulumi preview    # No --policy-pack flag needed
```

## Troubleshooting

See [references/troubleshooting.md](references/troubleshooting.md) for common
failure modes, PATH issues, version mismatches, plugin isolation, and config
tuning.

## Uninstall

```bash
finfocus analyzer uninstall              # Remove from Pulumi plugin dir
finfocus analyzer uninstall --target-dir /custom/path
```
