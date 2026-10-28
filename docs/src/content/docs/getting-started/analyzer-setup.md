---
title: Analyzer Setup
description: Setting up FinFocus as a Pulumi Analyzer Policy Pack
---

FinFocus integrates with Pulumi's analyzer framework as a **Policy Pack**. This allows you to see real-time cost
estimates directly within your `pulumi preview` and `pulumi up` workflow.

## Prerequisites

- [Pulumi CLI](https://www.pulumi.com/docs/get-started/install/) installed
- `finfocus` binary built (run `make build`)

## Setup Instructions

`finfocus analyzer install` creates the policy pack at `~/.finfocus/analyzer`.
The directory holds:

- `PulumiPolicy.yaml`
- `pulumi-analyzer-policy-finfocus`, which Pulumi before 3.270.0 starts
- `pulumi-language-finfocus`, which Pulumi 3.270.0 and later starts

On Windows both entries end in `.exe`.

```bash
finfocus analyzer install
```

An existing pack that has only `pulumi-analyzer-policy-finfocus` is updated by
running `finfocus analyzer install` again.

### Enable the Analyzer

The pack directory must be on your PATH, and the preview must point at that
directory. Pulumi 3.270.0 and later starts `pulumi-language-finfocus` from
PATH. Older releases start `pulumi-analyzer-policy-finfocus` the same way.

#### Option A: CLI Flag (Recommended for testing)

```bash
export PATH="${HOME}/.finfocus/analyzer:${PATH}"
pulumi preview --policy-pack ~/.finfocus/analyzer
```

#### Option B: Environment Variable (Recommended for CI/CD)

```bash
export PATH="${HOME}/.finfocus/analyzer:${PATH}"
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

### no language plugin 'pulumi-language-finfocus'

Pulumi 3.270.0 and later fails the preview when the language plugin entry is
missing:

```text
no language plugin 'pulumi-language-finfocus' found in the workspace or on your $PATH
```

Pulumi may also print a 404 while looking up a download named `pulumi-finfocus`
(`could not find latest version`). Re-run `finfocus analyzer install` and keep
`~/.finfocus/analyzer` on `PATH`.

### "could not start policy pack"

Ensure:

1. `~/.finfocus/analyzer/` contains `PulumiPolicy.yaml`,
   `pulumi-analyzer-policy-finfocus`, and `pulumi-language-finfocus`
   (both binaries end in `.exe` on Windows).
2. The directory is on your PATH: `export PATH="${HOME}/.finfocus/analyzer:${PATH}"`.
   Pulumi searches PATH for the entry it starts. Placing the files only in the
   policy pack directory is not sufficient.

### No cost diagnostics appear

1. Verify that `PulumiPolicy.yaml` exists in the same directory as the binary.
2. Ensure you are passing the correct path to `--policy-pack`.
3. Check logs by enabling debug mode:

   ```bash
   FINFOCUS_LOG_LEVEL=debug pulumi preview --policy-pack ~/.finfocus/analyzer
   ```

## Technical Details

For a deep dive into how the analyzer works, see the [Analyzer Integration Guide](../analyzer-integration.md).
