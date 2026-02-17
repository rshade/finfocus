---
title: Analyzer Setup
description: Setting up FinFocus as a Pulumi Analyzer Policy Pack
layout: default
---

FinFocus integrates with Pulumi's analyzer framework as a **Policy Pack**. This allows you to see real-time cost
estimates directly within your `pulumi preview` and `pulumi up` workflow.

## Prerequisites

- [Pulumi CLI](https://www.pulumi.com/docs/get-started/install/) installed
- `finfocus` binary built (run `make build`)

## Setup Instructions

Pulumi expects analyzer policy packs to follow a specific naming and directory convention.

### 1. Create a Policy Pack Directory

Create a dedicated directory to hold your FinFocus policy pack:

```bash
mkdir -p ~/.finfocus/analyzer
```

### 2. Configure the Policy Pack

Create a `PulumiPolicy.yaml` file in that directory. This file tells Pulumi to use the `finfocus` runtime.

```bash
cat > ~/.finfocus/analyzer/PulumiPolicy.yaml << 'EOF'
runtime: finfocus
name: finfocus
version: 0.3.0
EOF
```

### 3. Install the Binary

Copy your `finfocus` binary to the policy pack directory, renaming it to match Pulumi's expected naming convention:
`pulumi-analyzer-policy-<runtime>`.

```bash
# From the finfocus root directory
cp bin/finfocus ~/.finfocus/analyzer/pulumi-analyzer-policy-finfocus
chmod +x ~/.finfocus/analyzer/pulumi-analyzer-policy-finfocus
```

### 4. Enable the Analyzer

To use the analyzer, you must point Pulumi to the policy pack directory during preview or update.

#### Option A: CLI Flag (Recommended for testing)

```bash
pulumi preview --policy-pack ~/.finfocus/analyzer
```

#### Option B: Environment Variable (Recommended for CI/CD)

```bash
export PULUMI_POLICY_PACK_PATH="$HOME/.finfocus/analyzer"
pulumi preview
```

## Usage

Once configured, cost estimates will appear as **advisory diagnostics** in your Pulumi output:

```text
Policies:
    finfocus@v0.3.0 (local: ~/.finfocus/analyzer)
        - [advisory] [severity: low]  cost-estimate  (aws:ec2/instance:Instance: my-instance)
          Estimated Monthly Cost: $7.50 USD (source: finfocus-plugin-aws)
        - [advisory] [severity: low]  stack-cost-summary  (pulumi:pulumi:Stack: my-stack)
          Total Estimated Monthly Cost: $7.50 USD (1 resources analyzed)
```

## Troubleshooting

### "could not start policy pack"

Ensure the binary name in `~/.finfocus/analyzer/` matches exactly: `pulumi-analyzer-policy-finfocus`.

### No cost diagnostics appear

1. Verify that `PulumiPolicy.yaml` exists in the same directory as the binary.
2. Ensure you are passing the correct path to `--policy-pack`.
3. Check logs by enabling debug mode:

   ```bash
   FINFOCUS_LOG_LEVEL=debug pulumi preview --policy-pack ~/.finfocus/analyzer
   ```

## Technical Details

For a deep dive into how the analyzer works, see the [Analyzer Integration Guide](../analyzer-integration.md).
